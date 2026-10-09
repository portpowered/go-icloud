package contracts_test

import (
	"github.com/portpowered/go-icloud/internal/protocol"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestReminderLocationRequestContract(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	models := loadDriveDocument(t, cloudKitModelsPath)
	endpoint := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/modify").Post

	for _, name := range []string{"success", "record-error"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pair := accountExchanges(t, "../replay/fixtures/synthetic/http/reminders-add-location-trigger-"+name+".json")[0]

			value, err := driveJSONValue(pair.Request.Body)
			if err != nil {
				t.Fatal(err)
			}

			for _, schema := range []*openapi3.Schema{models.Components.Schemas["ReminderLocationRequest"].Value,
				endpoint.RequestBody.Value.Content["application/json"].Schema.Value} {
				err = schema.VisitJSON(value)
				if err != nil {
					t.Fatal(err)
				}

				checkLocationOperationShape(t, schema, value)
				checkLocationFields(t, schema, value)
			}
		})
	}
}

func checkLocationOperationShape(t *testing.T, schema *openapi3.Schema, value any) {
	t.Helper()

	root, valid := value.(map[string]any)
	if !valid {
		t.Fatal("request is not an object")
	}

	operations, valid := root["operations"].([]any)
	if !valid {
		t.Fatal("operations is not an array")
	}

	for index, original := range operations {
		operations[index] = operations[(index+1)%len(operations)]

		if schema.VisitJSON(value) == nil {
			t.Fatal("duplicate operation bypassed the location request variant")
		}

		operations[index] = original
	}

	root["atomic"] = false

	if schema.VisitJSON(value) == nil {
		t.Fatal("non-atomic location request accepted")
	}

	root["atomic"] = true
}

func checkLocationFields(t *testing.T, schema *openapi3.Schema, value any) {
	t.Helper()

	root, valid := value.(map[string]any)
	if !valid {
		t.Fatal("request is not an object")
	}

	operations, valid := root["operations"].([]any)
	if !valid {
		t.Fatal("operations is not an array")
	}

	for _, operation := range operations {
		operationObject, valid := operation.(map[string]any)
		if !valid {
			t.Fatal("operation is not an object")
		}

		record, valid := operationObject["record"].(map[string]any)
		if !valid {
			t.Fatal("record is not an object")
		}

		fields, valid := record["fields"].(map[string]any)
		if !valid {
			t.Fatal("fields is not an object")
		}

		for key, original := range fields {
			delete(fields, key)

			if schema.VisitJSON(value) == nil {
				t.Fatalf("missing %s accepted", key)
			}

			fields[key] = map[string]any{protocol.RemindersCKAssetFieldType: "INVALID", reminderWriteValue: true}

			if schema.VisitJSON(value) == nil {
				t.Fatalf("invalid %s wrapper accepted", key)
			}

			fields[key] = original
		}
	}
}
