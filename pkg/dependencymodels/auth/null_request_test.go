package auth_test

import (
	"encoding/json"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

func TestSessionValidationRequestPermitsOnlyNull(t *testing.T) {
	var request auth.AuthSessionValidationRequest
	encoded, err := json.Marshal(request)
	if err != nil || string(encoded) != "null" {
		t.Fatalf("null request encoding: %s, %v", encoded, err)
	}
	for _, input := range []string{"null", " \n\t null \r"} {
		decodeErr := json.Unmarshal([]byte(input), &request)
		if decodeErr != nil {
			t.Errorf("valid null %q rejected: %v", input, decodeErr)
		}
	}
	for _, input := range []string{"{}", "[]", "1", "true", `"null"`, "", "null null", "null {}"} {
		decodeErr := json.Unmarshal([]byte(input), &request)
		if decodeErr == nil {
			t.Errorf("invalid null request %q accepted", input)
		}
	}
}
