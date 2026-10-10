package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testDirectoryMode = 0o700
	testFileMode      = 0o600
	testContentOrigin = "https://content.invalid"
	testInventory     = `{"format":"portos.reference-wire-inventory.v1",
"source":{"url":"https://example.invalid/reference","commit":"synthetic-pin"},
"entries":[{"service":"drive","transport":"http","method":"GET","path":"/ws/{zone}/download/by_id"}]}`
	testScenario = `{"format":"portos.service-scenario.v1",
"source":{"url":"https://example.invalid/reference","commit":"synthetic-pin"},
"evidence":"synthetic; implementation-derived","service":"drive","inputs":[],
"exchanges":[{"request":{"method":"GET","origin":"https://example.invalid","path":"/ws/zone/download/by_id"},
"response":{"status":200,"body":{"encoding":"base64","value":"e30="}}}]}`
)

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()

	filename := filepath.Join(root, relative)

	err := os.MkdirAll(filepath.Dir(filename), testDirectoryMode)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filename, []byte(content), testFileMode)
	if err != nil {
		t.Fatal(err)
	}
}

func setupTestRepository(t *testing.T, definitions, document string) string {
	t.Helper()

	root := t.TempDir()
	writeTestFile(t, root, filepath.Join("docs", "reference-endpoints.json"), definitions)
	writeTestFile(t, root, filepath.Join("tools", "reference", "source.json"),
		`{"live":{"url":"https://example.invalid/reference","commit":"synthetic-pin"}}`)
	writeTestFile(t, root, filepath.Join("tests", "replay", "fixtures", "synthetic", "http", "case.json"), document)

	return root
}

func TestTemplateMethodAndServiceBinding(t *testing.T) {
	t.Parallel()

	for _, replacement := range []string{http.MethodDelete, "photos", "/ws/zone/extra/download/by_id", "200"} {
		t.Run(replacement, func(t *testing.T) {
			t.Parallel()

			modified := modifyScenario(replacement)
			root := setupTestRepository(t, testInventory, modified)

			report, err := audit(root)
			if err != nil {
				t.Fatal(err)
			}

			want := 1
			if replacement == "200" {
				want = 0
			}

			if len(report.Unmatched) != want || len(report.Missing) != want {
				t.Fatalf("wrong route disposition: %+v", report)
			}
		})
	}
}

func modifyScenario(replacement string) string {
	switch replacement {
	case http.MethodDelete:
		return strings.Replace(testScenario, `"method":"GET"`, `"method":"DELETE"`, 1)
	case "photos":
		return strings.Replace(testScenario, `"service":"drive"`, `"service":"photos"`, 1)
	case "200":
		return testScenario
	default:
		return strings.Replace(testScenario, "/ws/zone/download/by_id", replacement, 1)
	}
}

func TestInvalidEvidenceAndSourceFail(t *testing.T) {
	t.Parallel()

	for _, token := range []string{scenarioFormat, syntheticClass, "synthetic-pin"} {
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			modified := strings.Replace(testScenario, token, "wrong", 1)
			root := setupTestRepository(t, testInventory, modified)

			_, err := audit(root)
			if !errors.Is(err, errScenario) {
				t.Fatalf("invalid scenario accepted: %v", err)
			}
		})
	}
}

func TestDuplicateAndUnknownTemplatesFail(t *testing.T) {
	t.Parallel()

	for _, template := range []string{"/ws/{unknown}/file", "/ws/{zone/file", "/prefix/{provider-returned-url}"} {
		t.Run(template, func(t *testing.T) {
			t.Parallel()

			definitions := strings.Replace(testInventory, "/ws/{zone}/download/by_id", template, 1)
			root := setupTestRepository(t, definitions, testScenario)

			_, err := audit(root)
			if err == nil {
				t.Fatal("unregistered or malformed template accepted")
			}
		})
	}

	var definitions inventory

	err := json.Unmarshal([]byte(testInventory), &definitions)
	if err != nil {
		t.Fatal(err)
	}

	definitions.Entries = append(definitions.Entries, definitions.Entries[0])

	_, err = initializeReport(definitions.Entries)
	if !errors.Is(err, errEndpoint) {
		t.Fatalf("duplicate endpoint accepted: %v", err)
	}
}

func TestProviderURLMustPrecedeExchange(t *testing.T) {
	t.Parallel()

	var document scenario

	err := json.Unmarshal([]byte(testScenario), &document)
	if err != nil {
		t.Fatal(err)
	}

	document.Exchanges[0].Response.Body.Value = base64.StdEncoding.EncodeToString(
		[]byte(`{"token":{"url":"https://content.invalid/file?token=synthetic"}}`),
	)
	request := wireRequest{Method: "GET", Origin: testContentOrigin, Path: "/file"}

	if matchesRoute(providerURLTemplate, request, document, 0) {
		t.Fatal("future response URL accepted")
	}

	if !matchesRoute(providerURLTemplate, request, document, 1) {
		t.Fatal("preceding response URL missed")
	}

	request.Origin = "https://other.invalid"
	if matchesRoute(providerURLTemplate, request, document, 1) {
		t.Fatal("different authority accepted")
	}

	request.Origin, request.Path = testContentOrigin, "/file/extra"
	if matchesRoute(providerURLTemplate, request, document, 1) {
		t.Fatal("different escaped path accepted")
	}
}

func TestAmbiguousRouteFails(t *testing.T) {
	t.Parallel()

	definitions := strings.Replace(testInventory, `"entries":[`, `"entries":[
{"service":"drive","transport":"http","method":"GET","path":"/ws/zone/download/by_id"},`, 1)
	root := setupTestRepository(t, definitions, testScenario)

	_, err := audit(root)
	if !errors.Is(err, errAmbiguous) {
		t.Fatalf("ambiguous route accepted: %v", err)
	}
}

func TestRepositoryOccurrenceAudit(t *testing.T) {
	t.Parallel()

	report, err := audit(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	if report.Scenarios == 0 || report.Exchanges == 0 || len(report.Unmatched) != 0 {
		t.Fatalf("portable population not mapped: %+v", report)
	}

	if !strings.Contains(report.State, "not source/schema/behavior completeness") {
		t.Fatal("diagnostic report lost its scope limitation")
	}
}

func TestStrictCoverageRejectsMissingAndUnmatched(t *testing.T) {
	t.Parallel()

	root := setupTestRepository(t, testInventory, testScenario)

	report, err := audit(root)
	if err != nil {
		t.Fatal(err)
	}

	err = verifyCoverage(report)
	if err != nil {
		t.Fatal(err)
	}

	var summary bytes.Buffer

	err = writeReport(&summary, report, true)
	if err != nil || !strings.Contains(summary.String(), "1/1 HTTP routes") {
		t.Fatalf("wrong diagnostic summary: %s %v", summary.String(), err)
	}

	report.Missing = append(report.Missing, report.Routes[0].Endpoint)

	err = verifyCoverage(report)
	if !errors.Is(err, errIncomplete) {
		t.Fatalf("missing route accepted: %v", err)
	}

	report.Missing = nil
	report.Unmatched = append(report.Unmatched, unmatchedExchange{
		Fixture: "case.json", Exchange: 0,
		Request: wireRequest{Method: http.MethodDelete, Origin: "https://example.invalid", Path: "/unknown"},
	})

	err = verifyCoverage(report)
	if !errors.Is(err, errIncomplete) {
		t.Fatalf("unmatched route accepted: %v", err)
	}
}

func TestInventoryPinMustMatchActiveReference(t *testing.T) {
	t.Parallel()

	definitions := strings.Replace(testInventory, "synthetic-pin", "wrong", 1)
	root := setupTestRepository(t, definitions, strings.Replace(testScenario, "synthetic-pin", "wrong", 1))

	_, err := audit(root)
	if !errors.Is(err, errInventory) {
		t.Fatalf("inventory and fixtures bypassed active reference pin: %v", err)
	}
}
