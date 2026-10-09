package contracts_test

import (
	"encoding/json"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	recurrenceContractType        = "type"
	recurrenceContractInvalidType = "INVALID"
	recurrenceContractStringType  = "STRING"
)

func TestReminderRecurrenceVariantsBindPairedRequests(t *testing.T) {
	t.Parallel()
	models := loadDriveDocument(t, cloudKitModelsPath)
	document := loadDriveDocument(t, remindersSchemaPath)
	endpoint := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/modify").Post

	for variant, operation := range map[string]string{
		"ReminderRecurrenceCreationRequest": "create", "ReminderRecurrenceUpdateRequest": "update",
		"ReminderRecurrenceDeletionRequest": "delete",
	} {
		for _, outcome := range []string{"success", "record-error"} {
			t.Run(operation+outcome, func(t *testing.T) {
				t.Parallel()

				path := "../replay/fixtures/synthetic/http/reminders-" + operation + "-recurrence-rule-" + outcome + ".json"
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

					checkRecurrenceContractNegatives(t, schema, value)
				}
			})
		}
	}
}

func checkRecurrenceContractNegatives(t *testing.T, schema *openapi3.Schema, value any) {
	t.Helper()

	root := recurrenceContractObject(t, value)

	operations, ok := root["operations"].([]any)
	if !ok || len(operations) == 0 {
		t.Fatal("recurrence operations absent")
	}

	operation := recurrenceContractObject(t, operations[len(operations)-1])
	child := recurrenceContractObject(t, operation["record"])

	fields := recurrenceContractObject(t, child["fields"])
	checkRecurrenceFieldNegatives(t, schema, value, fields)

	zone := root["zoneID"]
	delete(root, "zoneID")

	if schema.VisitJSON(value) == nil {
		t.Fatal("missing recurrence zone accepted")
	}

	root["zoneID"] = zone
	if len(operations) > 1 {
		root["operations"] = []any{operations[0], operations[0]}

		if schema.VisitJSON(value) == nil {
			t.Fatal("duplicate recurrence parent accepted")
		}

		root["operations"] = []any{operations[1], operations[1]}

		if schema.VisitJSON(value) == nil {
			t.Fatal("duplicate recurrence child accepted")
		}

		root["operations"] = operations
	} else {
		checkRecurrenceNoSettings(t, schema, root, fields)
	}
}

func checkRecurrenceNoSettings(t *testing.T, schema *openapi3.Schema, root, fields map[string]any) {
	t.Helper()

	saved, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"Frequency", "Interval", "OccurrenceCount", "FirstDayOfTheWeek"} {
		delete(fields, key)
	}

	if schema.VisitJSON(root) == nil {
		t.Fatal("recurrence update without settings accepted")
	}

	err = json.Unmarshal(saved, &fields)
	if err != nil {
		t.Fatal(err)
	}
}

func recurrenceContractObject(t *testing.T, value any) map[string]any {
	t.Helper()

	object, ok := value.(map[string]any)
	if !ok {
		t.Fatal("recurrence contract requires an object")
	}

	return object
}

func checkRecurrenceFieldNegatives(t *testing.T, schema *openapi3.Schema, value any, fields map[string]any) {
	t.Helper()

	for key, original := range fields {
		fields[key] = map[string]any{recurrenceContractType: recurrenceContractInvalidType, reminderWriteValue: false}

		if schema.VisitJSON(value) == nil {
			t.Fatalf("invalid recurrence wrapper %s accepted", key)
		}

		fields[key] = original
	}

	reference := fields["Reminder"]
	if reference != nil {
		fields["Reminder"] = map[string]any{recurrenceContractType: "REFERENCE"}

		if schema.VisitJSON(value) == nil {
			t.Fatal("recurrence reference missing value accepted")
		}

		fields["Reminder"] = reference
	}

	fields["UnknownKnownField"] = map[string]any{
		recurrenceContractType: recurrenceContractStringType, reminderWriteValue: "invalid",
	}

	if schema.VisitJSON(value) == nil {
		t.Fatal("unregistered recurrence field accepted")
	}

	delete(fields, "UnknownKnownField")
}
