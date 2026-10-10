package command_test

import (
	"encoding/json"
	"testing"
)

func writeFixtureValue(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	writeProbeFile(t, path, data)
}
