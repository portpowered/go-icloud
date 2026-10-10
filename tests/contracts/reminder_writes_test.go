package contracts_test

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const reminderWriteValue = "value"

func TestReminderWriteVariantsBindCompleteRequests(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	models := loadDriveDocument(t, cloudKitModelsPath)
	endpoint := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/modify").Post

	for variant, names := range map[string][]string{
		reminderCreationRequestSchema: {reminderCreateBasicFixture, "create-completed", "create-dated-child",
			"create-lookup-empty", "create-record-error"},
		reminderUpdateRequestSchema: {
			reminderUpdateBasicFixture, "update-completed", "update-dated-child", "update-record-error",
		},
		reminderDeletionRequestSchema: {reminderDeleteFixture, "delete-record-error"},
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

					checkReminderWriteNegatives(t, schema, value)
				}

				checkEmbeddedReminderTokens(t, models, variant, value)
			})
		}
	}
}

func reminderWriteFields(t *testing.T, value any) map[string]any {
	t.Helper()

	root, rootOK := value.(map[string]any)
	if !rootOK {
		t.Fatal("mutation request is not an object")
	}

	operations, operationsOK := root["operations"].([]any)
	if !operationsOK || len(operations) != 1 {
		t.Fatal("mutation request has wrong operation shape")
	}

	operation, operationOK := operations[0].(map[string]any)
	if !operationOK {
		t.Fatal("mutation operation is not an object")
	}

	record, recordOK := operation["record"].(map[string]any)
	if !recordOK {
		t.Fatal("mutation record is not an object")
	}

	fields, ok := record["fields"].(map[string]any)
	if !ok {
		t.Fatal("mutation fields are not an object")
	}

	return fields
}

func checkReminderWriteNegatives(t *testing.T, schema *openapi3.Schema, value any) {
	t.Helper()

	fields := reminderWriteFields(t, value)
	for key, original := range fields {
		fields[key] = map[string]any{reminderContractWrapperType: "INVALID", reminderWriteValue: true}

		if schema.VisitJSON(value) == nil {
			t.Fatalf("invalid known %s wrapper accepted", key)
		}

		fields[key] = original
	}

	for _, key := range []string{reminderTitleDocumentField, reminderNotesDocumentField, "ResolutionTokenMap"} {
		original, exists := fields[key]
		if !exists {
			continue
		}

		for _, invalid := range []any{nil, true, map[string]any{"type": "STRING", reminderWriteValue: nil}} {
			fields[key] = invalid

			if schema.VisitJSON(value) == nil {
				t.Fatalf("invalid known %s value accepted", key)
			}
		}

		fields[key] = original
	}

	checkMissingReminderWriteFields(t, schema, value)
}

func checkMissingReminderWriteFields(t *testing.T, schema *openapi3.Schema, value any) {
	t.Helper()
	fields := reminderWriteFields(t, value)
	missing := make(map[string]any)

	for _, key := range []string{reminderTitleDocumentField, reminderNotesDocumentField, "Deleted"} {
		if field, exists := fields[key]; exists {
			missing[key] = field

			delete(fields, key)
		}
	}

	if schema.VisitJSON(value) == nil {
		t.Fatal("removal of known write fields entered pending linked-record fallback")
	}

	root, rootOK := value.(map[string]any)
	if !rootOK {
		t.Fatal("mutation request is not an object")
	}

	operations, operationsOK := root["operations"].([]any)
	if !operationsOK {
		t.Fatal("mutation request has wrong operation shape")
	}

	root["operations"] = append(operations, operations[0])

	if schema.VisitJSON(value) == nil {
		t.Fatal("malformed multi-operation write entered pending linked-record fallback")
	}

	root["operations"] = operations

	maps.Copy(fields, missing)
}

func checkEmbeddedReminderTokens(t *testing.T, document *openapi3.T, variant string, value any) {
	t.Helper()
	fields := reminderWriteFields(t, value)

	wrapper, wrapperOK := fields["ResolutionTokenMap"].(map[string]any)
	if !wrapperOK {
		t.Fatal("resolution tokens missing")
	}

	text, ok := wrapper[reminderWriteValue].(string)
	if !ok {
		t.Fatal("resolution tokens are not embedded JSON")
	}

	var tokens any

	err := json.Unmarshal([]byte(text), &tokens)
	if err != nil {
		t.Fatal(err)
	}

	component := map[string]string{reminderCreationRequestSchema: reminderCreationTokensSchema,
		reminderUpdateRequestSchema:   reminderUpdateTokensSchema,
		reminderDeletionRequestSchema: reminderDeletionResolutionSchema}[variant]
	schema := document.Components.Schemas[component].Value

	err = schema.VisitJSON(tokens)
	if err != nil {
		t.Fatal(err)
	}

	for _, invalid := range []any{nil, true, map[string]any{reminderContractTokenMap: map[string]any{}},
		map[string]any{reminderContractTokenMap: tokens, reminderFutureTokensField: true}} {
		if schema.VisitJSON(invalid) == nil {
			t.Fatal("invalid embedded token payload accepted")
		}
	}
}
