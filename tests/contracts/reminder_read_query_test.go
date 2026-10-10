package contracts_test

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestReminderReadQueryEndpointRejectsImpossibleFilters(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	schema := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/query").Post.
		RequestBody.Value.Content["application/json"].Schema.Value
	pair := accountExchanges(t, "../replay/fixtures/synthetic/http/reminders-query-1-active.json")[0]

	value, err := driveJSONValue(pair.Request.Body)
	if err != nil {
		t.Fatal(err)
	}

	err = schema.VisitJSON(value)
	if err != nil {
		t.Fatal(err)
	}

	root := hashtagContractObject(t, value)
	query := hashtagContractObject(t, root["query"])
	filters, ok := query["filterBy"].([]any)

	if !ok {
		t.Fatal("query filters are not an array")
	}

	for index, input := range filters {
		checkReminderReadFilterValue(t, schema, value, input)

		for _, duplicates := range [][]any{{input, input, input}, {input, input, filters[(index+1)%len(filters)]}} {
			query["filterBy"] = duplicates

			if schema.VisitJSON(value) == nil {
				t.Fatal("duplicate filter kind accepted")
			}
		}

		query["filterBy"] = filters
	}
}

func checkReminderReadFilterValue(t *testing.T, schema *openapi3.Schema, value, input any) {
	t.Helper()
	filter := hashtagContractObject(t, input)
	wrapper := hashtagContractObject(t, filter["fieldValue"])
	previous := wrapper["value"]

	if filter["fieldName"] == "List" {
		return
	}

	invalidValues := []any{nil, -1, 2, "1", false}
	if filter["fieldName"] == "LookupValidatingReference" {
		invalidValues = append(invalidValues, 0)
	}

	for _, invalid := range invalidValues {
		wrapper["value"] = invalid

		if schema.VisitJSON(value) == nil {
			t.Fatalf("invalid %s value accepted: %v", filter["fieldName"], invalid)
		}
	}

	wrapper["value"] = previous
}

func TestReminderSyncQueryEndpointRequiresFixedLimit(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	schema := document.Paths.Value("/database/1/com.apple.reminders/production/private/records/query").Post.
		RequestBody.Value.Content["application/json"].Schema.Value
	pair := accountExchanges(t, "../replay/fixtures/synthetic/http/reminders-sync-query.json")[0]

	value, err := driveJSONValue(pair.Request.Body)
	if err != nil {
		t.Fatal(err)
	}

	err = schema.VisitJSON(value)
	if err != nil {
		t.Fatal(err)
	}

	root := hashtagContractObject(t, value)
	for _, invalid := range []any{nil, 0, 2, "1"} {
		root["resultsLimit"] = invalid

		if schema.VisitJSON(value) == nil {
			t.Fatalf("invalid current-token limit accepted: %v", invalid)
		}
	}
}

func TestReminderZoneListEndpointRejectsAdditionalRequestFields(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	schema := document.Paths.Value("/database/1/com.apple.reminders/production/private/zones/list").Post.
		RequestBody.Value.Content["application/json"].Schema.Value
	value := map[string]any{}

	err := schema.VisitJSON(value)
	if err != nil {
		t.Fatal(err)
	}

	value["zoneName"] = "Reminders"
	if schema.VisitJSON(value) == nil {
		t.Fatal("zone discovery selection field accepted")
	}
}
