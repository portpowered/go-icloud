package main_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	gateTimeout        = 30 * time.Second
	guideTitle         = "Expected guide"
	rootContent        = "root"
	missingDestination = "missing local destination"
	missingContent     = "expected rendered content absent"
)

type gateScenario struct {
	name         string
	root         string
	guide        string
	externalDocs string
	failure      string
}

func gateScenarios() []gateScenario {
	return []gateScenario{
		{"valid", `<a href="/go-icloud/docs/guide/#details">guide</a>`,
			`<h1>Expected guide</h1><h2 id="details">Details</h2>`, "", ""},
		{"root-link", `<a href="/go-icloud/missing/">missing</a>`, guideTitle, "", missingDestination},
		{"reference-link", rootContent, `<h1>Expected guide</h1><a href="/go-icloud/missing/">bad</a>`,
			"", missingDestination},
		{"anchor", `<a href="/go-icloud/docs/guide/#missing">bad</a>`, guideTitle, "", "missing fragment"},
		{"runtime-schema-link", rootContent, guideTitle, "/go-icloud/missing/", missingDestination},
		{"fallback-content", rootContent, "Not found", "", missingContent},
		{"hidden-content", rootContent, `<script>Expected guide</script><h1>Not found</h1>`,
			"", missingContent},
	}
}

func TestRenderedSiteGate(t *testing.T) {
	t.Parallel()

	for _, scenario := range gateScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			writeGateFile(t, directory, "site/index.html", scenario.root)
			writeGateFile(t, directory, "site/docs/guide/index.html", scenario.guide)

			schema := `{"openapi":"3.0.3","info":{"title":"test","version":"1"},"paths":{}}`
			if scenario.externalDocs != "" {
				schema = strings.TrimSuffix(schema, "}") + `,"externalDocs":{"url":"` + scenario.externalDocs + `"}}`
			}

			writeGateFile(t, directory, "api/test.openapi.yaml", schema)

			expected, err := json.Marshal(map[string][]string{"/go-icloud/docs/guide": {guideTitle}})
			if err != nil {
				t.Fatal(err)
			}

			writeGateFile(t, directory, "expect.json", string(expected))

			output, err := runGate(t, directory)
			if scenario.failure == "" {
				if err != nil {
					t.Fatalf("valid site rejected: %v\n%s", err, output)
				}

				return
			}

			if err == nil || !strings.Contains(string(output), scenario.failure) {
				t.Fatalf("want %q rejection; got %v\n%s", scenario.failure, err, output)
			}
		})
	}
}

func TestChannelSchemaDocumentationLink(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	writeGateFile(t, directory, "site/index.html", rootContent)
	writeGateFile(t, directory, "expect.json", `{"/go-icloud":["root"]}`)
	writeGateFile(t, directory, "api/bridge.asyncapi.yaml", `asyncapi: '3.0.0'
info:
  title: bridge
  version: '1'
channels:
  bridge:
    externalDocs:
      url: /go-icloud/missing-channel-reference/
`)

	output, err := runGate(t, directory)
	if err == nil || !strings.Contains(string(output), missingDestination) {
		t.Fatalf("missing channel schema link accepted: %v\n%s", err, output)
	}
}

func runGate(t *testing.T, directory string) ([]byte, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), gateTimeout)
	defer cancel()

	//nolint:gosec // G204: fixed Go executable, private test paths and no shell; exercise the shipped gate.
	command := exec.CommandContext(ctx, "go", "run", ".", "-site", filepath.Join(directory, "site"),
		"-schemas", filepath.Join(directory, "api"), "-expect", filepath.Join(directory, "expect.json"))

	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("run documentation gate: %w", err)
	}

	return output, nil
}

func writeGateFile(t *testing.T, root, name, content string) {
	t.Helper()

	file := filepath.Join(root, name)

	err := os.MkdirAll(filepath.Dir(file), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(file, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
