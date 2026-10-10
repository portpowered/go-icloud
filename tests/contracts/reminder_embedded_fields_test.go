package contracts_test

import (
	"encoding/json"
	"testing"
)

func TestReminderAllEmbeddedTokenModelsRejectMalformedValues(t *testing.T) {
	t.Parallel()
	models := loadDriveDocument(t, cloudKitModelsPath)

	for name, component := range map[string]string{
		reminderCreateBasicFixture:      reminderCreationTokensSchema,
		reminderUpdateBasicFixture:      reminderUpdateTokensSchema,
		reminderDeleteFixture:           reminderDeletionResolutionSchema,
		reminderCreateHashtagFixture:    "ReminderHashtagLinkTokensMap",
		reminderURLAttachmentFixture:    "ReminderAttachmentLinkTokensMap",
		reminderCreateRecurrenceFixture: "ReminderRecurrenceLinkTokensMap",
		reminderAddLocationFixture:      "ReminderAlarmLinkTokensMap",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pair := accountExchanges(t, "../replay/fixtures/synthetic/http/reminders-"+name+".json")[0]

			value, err := driveJSONValue(pair.Request.Body)
			if err != nil {
				t.Fatal(err)
			}

			fields := hashtagContractFields(t, hashtagContractOperations(t, hashtagContractObject(t, value))[0])
			wrapper := hashtagContractObject(t, fields["ResolutionTokenMap"])
			text, ok := wrapper[reminderWriteValue].(string)
			if !ok {
				t.Fatal("encoded token string absent")
			}

			var decoded any

			err = json.Unmarshal([]byte(text), &decoded)
			if err != nil {
				t.Fatal(err)
			}

			schema := models.Components.Schemas[component].Value
			err = schema.VisitJSON(decoded)
			if err != nil {
				t.Fatal(err)
			}

			inner := hashtagContractObject(t, hashtagContractObject(t, decoded)[reminderContractTokenMap])
			inner["unexpectedToken"] = true
			if schema.VisitJSON(decoded) == nil {
				t.Fatal("unknown embedded token key accepted")
			}

			delete(inner, "unexpectedToken")

			for _, token := range inner {
				hashtagContractObject(t, token)["counter"] = 2

				break
			}

			if schema.VisitJSON(decoded) == nil {
				t.Fatal("forged embedded counter accepted")
			}
		})
	}
}

func TestReminderEndpointRejectsInvalidDocumentBase64(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	schema := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/modify").Post.
		RequestBody.Value.Content["application/json"].Schema.Value
	pair := accountExchanges(t, "../replay/fixtures/synthetic/http/reminders-create-basic.json")[0]

	value, err := driveJSONValue(pair.Request.Body)
	if err != nil {
		t.Fatal(err)
	}

	fields := hashtagContractFields(t, hashtagContractOperations(t, hashtagContractObject(t, value))[0])
	wrapper := hashtagContractObject(t, fields[reminderTitleDocumentField])

	for _, invalid := range []string{"", "A", "AAA", "!!!!", "AAAA="} {
		wrapper[reminderWriteValue] = invalid

		if schema.VisitJSON(value) == nil {
			t.Fatalf("invalid document base64 accepted: %q", invalid)
		}
	}
}
