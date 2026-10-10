package contracts_test

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestReminderEndpointRejectsImpossibleStaticFields(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	schema := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/modify").Post.
		RequestBody.Value.Content["application/json"].Schema.Value

	for _, name := range []string{reminderCreateBasicFixture, "update-basic", "delete-success",
		reminderCreateHashtagFixture, "update-hashtag-success", "delete-hashtag-success",
		reminderCreateRecurrenceFixture, "update-recurrence-rule-success", "delete-recurrence-rule-success",
		"create-url-attachment-success", "delete-attachment-success", reminderAddLocationFixture} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pair := accountExchanges(t, "../replay/fixtures/synthetic/http/reminders-"+name+".json")[0]

			value, err := driveJSONValue(pair.Request.Body)
			if err != nil {
				t.Fatal(err)
			}

			if err = schema.VisitJSON(value); err != nil {
				t.Fatal(err)
			}

			checkReminderStaticOperations(t, schema, value, name)
		})
	}
}

func checkReminderStaticOperations(t *testing.T, schema *openapi3.Schema, value any, name string) {
	t.Helper()
	root := hashtagContractObject(t, value)
	operations := hashtagContractOperations(t, root)

	for _, operation := range operations {
		fields := hashtagContractFields(t, operation)
		checkReminderStaticFieldValues(t, schema, value, fields, name)
	}

	if len(operations) > 1 {
		for _, operation := range operations {
			same := make([]any, len(operations))
			for index := range same {
				same[index] = operation
			}

			root["operations"] = same
			if schema.VisitJSON(value) == nil {
				t.Fatal("duplicate atomic operation kind accepted")
			}
		}

		root["operations"] = operations
	}
}

func checkReminderStaticFieldValues(t *testing.T, schema *openapi3.Schema,
	value any, fields map[string]any, name string,
) {
	t.Helper()

	for key, input := range fields {
		wrapper := hashtagContractObject(t, input)
		previous := wrapper[reminderWriteValue]

		switch key {
		case "CreationDate", "LastModifiedDate":
			wrapper[reminderWriteValue] = nil
		case "Deleted":
			if strings.HasPrefix(name, "delete") {
				wrapper[reminderWriteValue] = 0
			} else {
				wrapper[reminderWriteValue] = 1
			}
		case "Imported", "Completed", "Flagged", "AllDay":
			wrapper[reminderWriteValue] = 2
		default:
			continue
		}

		if schema.VisitJSON(value) == nil {
			t.Fatalf("impossible static %s accepted", key)
		}

		wrapper[reminderWriteValue] = previous
	}

	for _, key := range []string{reminderRecordTypeName, "List"} {
		input, present := fields[key]
		if !present {
			continue
		}

		checkReminderRequiredReference(t, schema, value, hashtagContractObject(t, input))
	}
}

func checkReminderRequiredReference(t *testing.T, schema *openapi3.Schema, value any, wrapper map[string]any) {
	t.Helper()
	reference := hashtagContractObject(t, wrapper[reminderWriteValue])
	previous := reference["action"]
	reference["action"] = recurrenceContractInvalidType

	if schema.VisitJSON(value) == nil {
		t.Fatal("invalid reference action accepted")
	}

	reference["action"] = previous

	delete(wrapper, reminderWriteValue)

	if schema.VisitJSON(value) == nil {
		t.Fatal("missing required reference value accepted")
	}

	wrapper[reminderWriteValue] = reference
}
