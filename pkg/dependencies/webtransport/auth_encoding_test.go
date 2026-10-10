package webtransport

import (
	"bytes"
	"testing"

	bridgemodels "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
)

func TestBridgeOpaqueEncodingRetainsOpenProviderJSON(t *testing.T) {
	t.Parallel()
	for _, control := range []struct {
		name     string
		input    any
		expected string
	}{
		{"null", nil, ""},
		{"object", map[string]any{"name": "source"}, `"{\"name\":\"source\"}"`},
		{"array", []any{"source", true}, `["source",true]`},
		{"string", "source", `"source"`},
		{"boolean", true, "true"},
		{"number", 1.5, "1.5"},
	} {
		t.Run(control.name, func(t *testing.T) {
			t.Parallel()
			// The field and helper retain compatibility with existing any values.
			input := bridgemodels.BridgeExchange{Akdata: control.input}
			actual, err := EncodeBridgeOpaqueData(input.Akdata)
			if err != nil || !bytes.Equal(actual, []byte(control.expected)) {
				t.Fatalf("opaque provider encoding=%s error=%v; expected=%s", actual, err, control.expected)
			}
		})
	}
}
