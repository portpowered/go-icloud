package contracts_test

import (
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/portpowered/go-icloud/internal/protocol"
)

func TestReminderAttachmentWriteSchemas(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	models := loadDriveDocument(t, cloudKitModelsPath)
	endpoint := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/modify").Post

	for variant, names := range map[string][]string{
		"ReminderAttachmentURLCreationRequest": {"create-url-attachment-success", "create-url-attachment-record-error"},
		"ReminderAttachmentURLUpdateRequest":   {"update-attachment-success", "update-attachment-record-error"},
		"ReminderAttachmentImageUpdateRequest": {"update-image-success"},
		"ReminderAttachmentDeletionRequest": {"delete-attachment-success", "delete-attachment-record-error",
			"delete-attachment-empty-id-success"},
	} {
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				pair := accountExchanges(t, "../replay/fixtures/synthetic/http/reminders-"+name+".json")[0]

				value, err := driveJSONValue(pair.Request.Body)
				if err != nil {
					t.Fatal(err)
				}

				for _, schema := range []*openapi3.Schema{models.Components.Schemas[variant].Value,
					endpoint.RequestBody.Value.Content["application/json"].Schema.Value} {
					err = schema.VisitJSON(value)
					if err != nil {
						t.Fatal(err)
					}

					attachmentSchemaNegatives(t, schema, value)
				}
			})
		}
	}
}

func attachmentSchemaNegatives(t *testing.T, schema *openapi3.Schema, value any) {
	t.Helper()

	for index, mutation := range []func(map[string]any){
		func(root map[string]any) { delete(root, "zoneID") },
		func(root map[string]any) {
			record := attachmentSchemaRecord(t, root)

			fields, ok := record["fields"].(map[string]any)
			if !ok {
				t.Fatal("attachment fields are not an object")
			}

			fields["unregisteredAttachmentField"] = map[string]any{
				protocol.RemindersCKStringFieldType: "STRING", "value": "unregistered"}
		},
		func(root map[string]any) {
			attachmentSchemaRecord(t, root)["fields"] = map[string]any{}
		},
		func(root map[string]any) {
			operations := attachmentSchemaOperations(t, root)
			if len(operations) == 2 {
				operations[1] = operations[0]
			} else {
				root["operations"] = append(operations, operations[0])
			}
		},
	} {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}

		var root map[string]any

		err = json.Unmarshal(body, &root)
		if err != nil {
			t.Fatal(err)
		}

		mutation(root)

		if schema.VisitJSON(root) == nil {
			t.Fatal("attachment schema accepted a malformed known mutation", index)
		}
	}
}

func attachmentSchemaOperations(t *testing.T, root map[string]any) []any {
	t.Helper()

	operations, ok := root["operations"].([]any)
	if !ok || len(operations) == 0 {
		t.Fatal("attachment operations are not a nonempty array")
	}

	return operations
}

func attachmentSchemaRecord(t *testing.T, root map[string]any) map[string]any {
	t.Helper()
	operations := attachmentSchemaOperations(t, root)

	operation, operationOK := operations[len(operations)-1].(map[string]any)
	if !operationOK {
		t.Fatal("attachment operation is not an object")
	}

	record, recordOK := operation["record"].(map[string]any)
	if !recordOK {
		t.Fatal("attachment record is not an object")
	}

	return record
}
