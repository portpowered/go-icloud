package replay_test

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
)

func TestBridgeHostSourcePairs(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("fixtures/synthetic/bridge-host.json")
	if err != nil {
		t.Fatal(err)
	}

	var fixture struct {
		Format string `json:"format"`
		Source struct {
			Commit string `json:"commit"`
		} `json:"source"`
		Cases []struct {
			Name         string                    `json:"name"`
			Input        models.BridgeInitiateData `json:"input"`
			Host         string                    `json:"host"`
			Error        bool                      `json:"error"`
			ErrorType    string                    `json:"errorType"`
			ErrorMessage string                    `json:"errorMessage"`
		} `json:"cases"`
	}

	err = json.Unmarshal(data, &fixture)
	if err != nil {
		t.Fatal(err)
	}

	if fixture.Format != "portos.bridge-host.v1" || fixture.Source.Commit != "e2e44ab875d47dab4475096021da60030f26c35e" || len(fixture.Cases) != 47 {
		t.Fatal("host fixture provenance or denominator changed")
	}

	for _, scenario := range fixture.Cases {
		t.Run(scenario.Name, func(t *testing.T) {
			t.Parallel()

			host, hostError := bridge.SocketHost(scenario.Input)
			if host != scenario.Host || (hostError != nil) != scenario.Error {
				t.Fatalf("host = %q, error = %v; Source host = %q, error = %t", host, hostError, scenario.Host, scenario.Error)
			}
			if scenario.ErrorType == "ValueError" {
				if hostError == nil || !strings.HasSuffix(hostError.Error(), scenario.ErrorMessage) || errors.Unwrap(hostError) == nil {
					t.Fatalf("Source parser error message/cause lost: %v; expected %q", hostError, scenario.ErrorMessage)
				}
			}
		})
	}
}
