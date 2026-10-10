package cloudkit_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func TestZoneDiscoveryRequestsHaveNoPayloadFields(t *testing.T) {
	t.Parallel()

	for _, request := range []any{cloudkit.CKEmptyRequest{}, cloudkit.ReminderZoneListRequest{}} {
		if value := reflect.TypeOf(request); value.Kind() != reflect.Struct || value.NumField() != 0 {
			t.Errorf("empty request has a caller payload surface: %v", value)
		}

		encoded, err := json.Marshal(request)
		if err != nil || string(encoded) != "{}" {
			t.Errorf("empty request encoding: %s, %v", encoded, err)
		}
	}
}
