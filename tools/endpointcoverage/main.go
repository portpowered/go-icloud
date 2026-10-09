// Command endpointcoverage audits portable HTTP scenarios against the draft source inventory.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/portpowered/go-icloud/internal/protocol"
)

const (
	inventoryFormat    = "portos.reference-wire-inventory.v1"
	scenarioFormat     = "portos.service-scenario.v1"
	syntheticClass     = "synthetic; implementation-derived"
	httpTransport      = "http"
	firstErrorStatus   = 400
	firstSuccessStatus = 200
)

var (
	errIncomplete  = errors.New("HTTP endpoint coverage is incomplete")
	errInventory   = errors.New("invalid source inventory format or pin")
	errNoScenarios = errors.New("no portable HTTP scenarios")
	errEndpoint    = errors.New("duplicate or incomplete endpoint")
	errNoRoutes    = errors.New("inventory has no HTTP routes")
	errScenario    = errors.New("invalid scenario format, source, evidence or service")
	errAmbiguous   = errors.New("ambiguous route binding")
)

type sourcePin struct {
	URL    string `json:"url"`
	Commit string `json:"commit"`
}

type referencePins struct {
	Live sourcePin `json:"live"`
}

type endpoint struct {
	Service   string `json:"service"`
	Transport string `json:"transport"`
	Method    string `json:"method"`
	Path      string `json:"path"`
}

type inventory struct {
	Format  string     `json:"format"`
	Source  sourcePin  `json:"source"`
	Entries []endpoint `json:"entries"`
}

type wireRequest struct {
	Method string `json:"method"`
	Origin string `json:"origin"`
	Path   string `json:"path"`
}

type entity struct {
	Encoding string `json:"encoding"`
	Value    string `json:"value"`
}

type wireResponse struct {
	Status int    `json:"status"`
	Body   entity `json:"body"`
}

type exchange struct {
	Request  wireRequest  `json:"request"`
	Response wireResponse `json:"response"`
}

type scenario struct {
	Format    string          `json:"format"`
	Source    sourcePin       `json:"source"`
	Evidence  string          `json:"evidence"`
	Service   string          `json:"service"`
	Inputs    json.RawMessage `json:"inputs"`
	Exchanges []exchange      `json:"exchanges"`
}

type occurrence struct {
	Fixture  string `json:"fixture"`
	Exchange int    `json:"exchange"`
	Status   int    `json:"status"`
}

type routeCoverage struct {
	Endpoint    endpoint     `json:"endpoint"`
	Occurrences []occurrence `json:"occurrences"`
	Successful  int          `json:"successful"`
	HTTPError   int          `json:"httpError"`
	OtherStatus int          `json:"otherStatus"`
}

type unmatchedExchange struct {
	Fixture  string      `json:"fixture"`
	Exchange int         `json:"exchange"`
	Request  wireRequest `json:"request"`
}

type coverageReport struct {
	State     string              `json:"state"`
	Scenarios int                 `json:"scenarios"`
	Exchanges int                 `json:"exchanges"`
	Routes    []routeCoverage     `json:"routes"`
	Missing   []endpoint          `json:"missing"`
	Unmatched []unmatchedExchange `json:"unmatched"`
}

func main() {
	err := run(os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output io.Writer) error {
	root := flag.String("root", ".", "repository root")
	requireCovered := flag.Bool("require-covered", false, "fail on missing HTTP routes or unmatched exchanges")
	summary := flag.Bool("summary", false, "print a compact occurrence and gap summary")

	flag.Parse()

	report, err := audit(*root)
	if err != nil {
		return err
	}

	err = writeReport(output, report, *summary)
	if err != nil {
		return fmt.Errorf("write endpoint coverage: %w", err)
	}

	if *requireCovered {
		return verifyCoverage(report)
	}

	return nil
}

func verifyCoverage(report coverageReport) error {
	if len(report.Missing) != 0 || len(report.Unmatched) != 0 {
		return errIncomplete
	}

	return nil
}

func writeReport(output io.Writer, report coverageReport, summary bool) error {
	if summary {
		_, err := fmt.Fprintf(output, "%s\n%d scenarios; %d exchanges; %d/%d HTTP routes; %d unmatched exchanges\n",
			report.State, report.Scenarios, report.Exchanges,
			len(report.Routes)-len(report.Missing), len(report.Routes), len(report.Unmatched))
		if err != nil {
			return fmt.Errorf("write endpoint summary: %w", err)
		}

		for _, route := range report.Missing {
			_, err = fmt.Fprintf(output, "missing: %s %s %s\n", route.Service, route.Method, route.Path)
			if err != nil {
				return fmt.Errorf("write missing endpoint: %w", err)
			}
		}

		return nil
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(report)
	if err != nil {
		return fmt.Errorf("encode endpoint report: %w", err)
	}

	return nil
}

func readJSON(filename string, target any) error {
	// #nosec G304 -- this offline tool reads explicitly selected repository files.
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}

	err = json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("decode %s: %w", filename, err)
	}

	return nil
}

func audit(root string) (coverageReport, error) {
	var definitions inventory

	err := readJSON(filepath.Join(root, "docs", "reference-endpoints.json"), &definitions)
	if err != nil {
		return coverageReport{}, err
	}

	if definitions.Format != inventoryFormat || definitions.Source.Commit == "" || definitions.Source.URL == "" {
		return coverageReport{}, errInventory
	}

	err = validatePin(root, definitions.Source)
	if err != nil {
		return coverageReport{}, err
	}

	report, err := initializeReport(definitions.Entries)
	if err != nil {
		return coverageReport{}, err
	}

	err = scanScenarios(root, &report, definitions.Source)
	if err != nil {
		return coverageReport{}, err
	}

	for _, route := range report.Routes {
		if len(route.Occurrences) == 0 {
			report.Missing = append(report.Missing, route.Endpoint)
		}
	}

	return report, nil
}

func validatePin(root string, pin sourcePin) error {
	var references referencePins

	err := readJSON(filepath.Join(root, "tools", "reference", "source.json"), &references)
	if err != nil {
		return err
	}

	if references.Live != pin {
		return errInventory
	}

	return nil
}

func scanScenarios(root string, report *coverageReport, pin sourcePin) error {
	files, err := filepath.Glob(filepath.Join(root, "tests", "replay", "fixtures", "synthetic", "http", "*.json"))
	if err != nil {
		return fmt.Errorf("list HTTP scenarios: %w", err)
	}

	if len(files) == 0 {
		return errNoScenarios
	}

	for _, filename := range files {
		err = auditScenario(report, filename, pin)
		if err != nil {
			return err
		}
	}

	return nil
}

func initializeReport(entries []endpoint) (coverageReport, error) {
	report := coverageReport{
		State:     "draft HTTP occurrence audit; not source/schema/behavior completeness or socket coverage",
		Scenarios: 0, Exchanges: 0, Routes: []routeCoverage{}, Missing: []endpoint{}, Unmatched: []unmatchedExchange{},
	}
	seen := make(map[endpoint]bool)

	for _, entry := range entries {
		if seen[entry] || entry.Service == "" || entry.Transport == "" || entry.Method == "" || entry.Path == "" {
			return coverageReport{}, fmt.Errorf("%w: %+v", errEndpoint, entry)
		}

		seen[entry] = true

		if entry.Transport == httpTransport {
			err := validateTemplate(entry.Path)
			if err != nil {
				return coverageReport{}, err
			}

			report.Routes = append(report.Routes, routeCoverage{
				Endpoint: entry, Occurrences: []occurrence{}, Successful: 0, HTTPError: 0, OtherStatus: 0,
			})
		}
	}

	if len(report.Routes) == 0 {
		return coverageReport{}, errNoRoutes
	}

	return report, nil
}

func auditScenario(report *coverageReport, filename string, pin sourcePin) error {
	var document scenario

	err := readJSON(filename, &document)
	if err != nil {
		return err
	}

	if document.Format != scenarioFormat || document.Source != pin ||
		document.Evidence != syntheticClass || document.Service == "" {
		return fmt.Errorf("%w: %s", errScenario, filename)
	}

	report.Scenarios++

	for index, pair := range document.Exchanges {
		err = recordExchange(report, filepath.Base(filename), index, document, pair)
		if err != nil {
			return err
		}
	}

	return nil
}

func recordExchange(report *coverageReport, filename string, index int, document scenario, pair exchange) error {
	report.Exchanges++
	matched := false
	service := exchangeService(document.Service, pair.Request)

	for routeIndex := range report.Routes {
		route := &report.Routes[routeIndex]
		if route.Endpoint.Service != service || route.Endpoint.Method != pair.Request.Method {
			continue
		}

		if !matchesRoute(route.Endpoint.Path, pair.Request, document, index) {
			continue
		}

		if matched {
			return fmt.Errorf("%w: %s exchange %d", errAmbiguous, filename, index)
		}

		matched = true

		route.Occurrences = append(route.Occurrences, occurrence{
			Fixture: filename, Exchange: index, Status: pair.Response.Status,
		})

		switch {
		case pair.Response.Status >= firstErrorStatus:
			route.HTTPError++
		case pair.Response.Status >= firstSuccessStatus:
			route.Successful++
		default:
			route.OtherStatus++
		}
	}

	if !matched {
		report.Unmatched = append(report.Unmatched, unmatchedExchange{
			Fixture: filename, Exchange: index, Request: pair.Request,
		})
	}

	return nil
}

// Find My's saved-token recovery crosses into the authentication service.
func exchangeService(service string, request wireRequest) string {
	if service == "findmy" && request.Method == http.MethodPost &&
		request.Path == protocol.AuthLoginAuthTokenPath && request.Origin == protocol.AuthAccountServer0 {
		return "auth"
	}

	return service
}
