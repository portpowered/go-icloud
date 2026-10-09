package contracts_test

import "testing"

func TestReminderRequiredTimestampsRejectNullAtEndpoint(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	schema := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/modify").Post.
		RequestBody.Value.Content["application/json"].Schema.Value

	for _, operation := range []string{"delete", "create-hashtag", "delete-hashtag",
		"create-recurrence-rule", "delete-recurrence-rule", "create-url-attachment", "delete-attachment",
		"add-location-trigger"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			path := "../replay/fixtures/synthetic/http/reminders-" + operation + "-success.json"
			pair := accountExchanges(t, path)[0]

			value, err := driveJSONValue(pair.Request.Body)
			if err != nil {
				t.Fatal(err)
			}

			if err = schema.VisitJSON(value); err != nil {
				t.Fatal(err)
			}

			root := recurrenceContractObject(t, value)
			operations, ok := root["operations"].([]any)
			if !ok || len(operations) == 0 {
				t.Fatal("timestamp operations absent")
			}

			parent := recurrenceContractObject(t, recurrenceContractObject(t, operations[0])["record"])
			fields := recurrenceContractObject(t, parent["fields"])
			stamp := recurrenceContractObject(t, fields["LastModifiedDate"])
			stamp["value"] = nil

			if schema.VisitJSON(value) == nil {
				t.Fatal("required timestamp accepted null")
			}
		})
	}
}
