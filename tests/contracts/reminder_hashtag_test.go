package contracts_test

import (
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	reminderContractWrapperType = "type"
	reminderContractTokenMap    = "map"
)

func hashtagContractObject(t *testing.T, value any) map[string]any {
	t.Helper()

	object, ok := value.(map[string]any)
	if !ok {
		t.Fatal("hashtag contract value is not an object")
	}

	return object
}

func hashtagContractOperations(t *testing.T, root map[string]any) []any {
	t.Helper()

	operations, ok := root["operations"].([]any)
	if !ok {
		t.Fatal("hashtag contract operations is not an array")
	}

	return operations
}

func hashtagContractFields(t *testing.T, operation any) map[string]any {
	t.Helper()
	object := hashtagContractObject(t, operation)
	record := hashtagContractObject(t, object["record"])

	return hashtagContractObject(t, record["fields"])
}

func TestReminderHashtagWritesBindCompleteRequests(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	models := loadDriveDocument(t, cloudKitModelsPath)
	endpoint := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/modify").Post

	for variant, operation := range map[string]string{
		"ReminderHashtagCreationRequest": "create", "ReminderHashtagUpdateRequest": "update",
		"ReminderHashtagDeletionRequest": "delete",
	} {
		for _, outcome := range []string{"success", "record-error"} {
			t.Run(operation+"/"+outcome, func(t *testing.T) {
				t.Parallel()

				path := "../replay/fixtures/synthetic/http/reminders-" + operation + "-hashtag-" + outcome + ".json"
				pair := accountExchanges(t, path)[0]

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

					checkHashtagWriteNegatives(t, schema, value)
				}

				if operation != "update" {
					checkHashtagEmbeddedTokens(t, models, value)
				}
			})
		}
	}
}

func checkHashtagWriteNegatives(t *testing.T, schema *openapi3.Schema, value any) {
	t.Helper()

	root := hashtagContractObject(t, value)

	operations := hashtagContractOperations(t, root)
	for _, operation := range operations {
		fields := hashtagContractFields(t, operation)
		for name, original := range fields {
			fields[name] = map[string]any{reminderContractWrapperType: "INVALID", "value": true}

			if schema.VisitJSON(value) == nil {
				t.Fatalf("known %s field escaped endpoint schema", name)
			}

			fields[name] = original
			if name == "Name" || name == "Deleted" || name == "HashtagIDs" {
				delete(fields, name)

				if schema.VisitJSON(value) == nil {
					t.Fatalf("missing %s escaped endpoint schema", name)
				}

				fields[name] = original
			}
		}
	}

	checkHashtagAtomicNegatives(t, schema, value)
}

func checkHashtagAtomicNegatives(t *testing.T, schema *openapi3.Schema, value any) {
	t.Helper()
	root := hashtagContractObject(t, value)
	operations := hashtagContractOperations(t, root)

	if len(operations) > 1 {
		root["atomic"] = false

		if schema.VisitJSON(value) == nil {
			t.Fatal("non-atomic hashtag relation accepted")
		}

		root["atomic"] = true
		for _, duplicate := range operations {
			root["operations"] = []any{duplicate, duplicate}

			if schema.VisitJSON(value) == nil {
				t.Fatal("hashtag relation without exactly one parent and child accepted")
			}
		}

		root["operations"] = operations[:1]

		if schema.VisitJSON(value) == nil {
			t.Fatal("orphaned hashtag parent update accepted")
		}

		root["operations"] = operations
	}
}

func checkHashtagEmbeddedTokens(t *testing.T, document *openapi3.T, value any) {
	t.Helper()

	root := hashtagContractObject(t, value)
	operations := hashtagContractOperations(t, root)
	fields := hashtagContractFields(t, operations[0])
	wrapper := hashtagContractObject(t, fields["ResolutionTokenMap"])

	text, ok := wrapper["value"].(string)
	if !ok {
		t.Fatal("embedded hashtag tokens is not a string")
	}

	var tokens any

	err := json.Unmarshal([]byte(text), &tokens)
	if err != nil {
		t.Fatal(err)
	}

	schema := document.Components.Schemas["ReminderHashtagLinkTokensMap"].Value

	err = schema.VisitJSON(tokens)
	if err != nil {
		t.Fatal(err)
	}

	for _, invalid := range []any{nil, true, map[string]any{reminderContractTokenMap: map[string]any{}},
		map[string]any{reminderContractTokenMap: tokens, "futureTokens": true}} {
		if schema.VisitJSON(invalid) == nil {
			t.Fatal("invalid embedded hashtag resolution tokens accepted")
		}
	}
}
