package command_test

import (
	"encoding/json"
	"testing"
)

func sourceServiceObject(t *testing.T, value any) map[string]any {
	t.Helper()

	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("Source value is not an object: %T", value)
	}

	return object
}

func sourceServiceList(t *testing.T, value any) []any {
	t.Helper()

	list, ok := value.([]any)
	if !ok {
		t.Fatalf("Source value is not a list: %T", value)
	}

	return list
}

func sourceServiceNumber(t *testing.T, value any) json.Number {
	t.Helper()

	number, ok := value.(json.Number)
	if !ok {
		t.Fatalf("Source value is not a JSON number: %T", value)
	}

	return number
}
