package webtransport

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestReferenceJSONEscapesPreservesValuesAndSourceSpelling(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "HTML", input: `{"value":"\u003c\u003C\u003e\u003E\u0026"}`, expected: `{"value":"<<>>&"}`},
		{name: "literal escape", input: `{"value":"\\u003c"}`, expected: `{"value":"\\u003c"}`},
		{name: "escaped quote", input: `{"value":"\"\u003c"}`, expected: `{"value":"\"<"}`},
		{name: "escaped slash", input: `{"value":"\\\u003c"}`, expected: `{"value":"\\<"}`},
		{name: "other escape", input: `{"value":"\u0041\u00e9"}`, expected: `{"value":"\u0041\u00e9"}`},
		{name: "member name", input: `{"\u003c\u003e\u0026":"\\u003c"}`, expected: `{"<>&":"\\u003c"}`},
		{name: "other values", input: `{"value":null,"number":7,"text":"é"}`,
			expected: `{"value":null,"number":7,"text":"é"}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual := referenceJSONEscapes([]byte(test.input))
			if string(actual) != test.expected {
				t.Fatalf("normalized JSON = %s, want %s", actual, test.expected)
			}

			var original, normalized any

			err := json.Unmarshal([]byte(test.input), &original)
			if err != nil {
				t.Fatal(err)
			}

			err = json.Unmarshal(actual, &normalized)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(original, normalized) {
				t.Fatal("JSON normalization changed member names or values")
			}
		})
	}
}
