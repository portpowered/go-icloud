package command_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sourceWriteProjection(value any) any {
	switch node := value.(type) {
	case []any:
		result := make([]any, 0, len(node))
		for _, child := range node {
			result = append(result, sourceWriteProjection(child))
		}
		return result
	case map[string]any:
		if text, wrapped := node["$datetime"].(string); wrapped {
			return sourceWriteDate(text)
		}
		if model, wrapped := node["$model"].(string); wrapped {
			result := sourceWriteProjection(node["value"])
			if model == "Reminder" {
				defaultWriteReminder(result.(map[string]any))
			}
			if model == "URLAttachment" {
				attachment := result.(map[string]any)
				if _, exists := attachment["uti"]; !exists {
					attachment["uti"] = "public.url"
				}
			}
			return result
		}
		result := make(map[string]any, len(node))
		for key, child := range node {
			name := sourceWriteField(key)
			if text, ok := child.(string); ok && writeDateField(name) {
				result[name] = sourceWriteDate(text)
			} else {
				result[name] = sourceWriteProjection(child)
			}
		}
		return result
	default:
		return value
	}
}

func sourceWriteField(key string) string {
	switch key {
	case "desc":
		return "description"
	case "fullname":
		return "fullName"
	case "asset":
		return "assetMetadata"
	}
	names := map[string]string{
		"list_id": "listID", "reminder_id": "reminderID", "parent_reminder_id": "parentReminderID",
		"hashtag_ids": "hashtagIDs", "attachment_ids": "attachmentIDs",
		"alarm_ids": "alarmIDs", "recurrence_rule_ids": "recurrenceRuleIDs",
		"alarm_id": "alarmID", "alarm_uid": "alarmUID", "trigger_id": "triggerID", "location_uid": "locationUID",
		"file_asset_url": "fileAssetURL",
		"master_id":      "masterID",
	}
	if name, known := names[key]; known {
		return name
	}
	parts := strings.Split(key, "_")

	for index := 1; index < len(parts); index++ {
		part := parts[index]
		if part != "" {
			parts[index] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

func defaultWriteReminder(value map[string]any) {
	for _, key := range []string{"alarmIDs", "attachmentIDs", "hashtagIDs", "recurrenceRuleIDs"} {
		if _, exists := value[key]; !exists {
			value[key] = []any{}
		}
	}
}

func writeDateField(key string) bool {
	switch key {
	case "added", "created", "modified", "completedDate", "dueDate", "startDate":
		return true
	default:
		return false
	}
}

func sourceWriteDate(text string) string {
	if instant, err := time.Parse(time.RFC3339Nano, text); err == nil {
		return instant.UTC().Format(time.RFC3339Nano)
	}
	if instant, err := time.Parse("2006-01-02T15:04:05", text); err == nil {
		return instant.UTC().Format(time.RFC3339Nano)
	}
	return text
}

func fixtureWriteInput(t *testing.T, operation string, fixture writeFixture) map[string]any {
	t.Helper()
	request := map[string]any{}
	if fixture.Keywords != nil {
		request = sourceWriteProjection(fixture.Keywords).(map[string]any)
	}
	inputs := sourceWriteProjection(fixture.Inputs).([]any)

	switch operation {
	case "reminder-create":
		request["listID"], request["title"] = inputs[0], inputs[1]
		if len(inputs) > 2 {
			request["description"] = inputs[2]
		}
	case "reminder-update":
		request["reminder"] = inputs[0]
	case "reminder-delete":
		reminder := inputs[0].(map[string]any)
		request["reminderID"], request["recordChangeTag"] = reminder["id"], reminder["recordChangeTag"]
	case "reminder-hashtag-create":
		request["reminder"], request["name"] = inputs[0], inputs[1]
	case "reminder-hashtag-update":
		request["hashtag"], request["name"] = inputs[0], inputs[1]
	case "reminder-hashtag-delete":
		request["reminder"], request["hashtag"] = inputs[0], inputs[1]
	case "reminder-recurrence-create":
		request["reminder"] = inputs[0]
	case "reminder-recurrence-update":
		request["recurrenceRule"] = inputs[0]
	case "reminder-recurrence-delete":
		request["reminder"], request["recurrenceRule"] = inputs[0], inputs[1]
	case "reminder-attachment-create":
		request["reminder"], request["url"] = inputs[0], inputs[1]
	case "reminder-attachment-update":
		request["attachment"] = inputs[0]
	case "reminder-attachment-delete":
		request["reminder"], request["attachment"] = inputs[0], inputs[1]
	case "reminder-location-add":
		request["reminder"] = inputs[0]
	default:
		photoWriteInput(t, request, operation, inputs)
	}
	return request
}

func photoWriteInput(t *testing.T, request map[string]any, operation string, inputs []any) {
	t.Helper()
	switch operation {
	case "photo-album-create":
		request["name"] = inputs[0]
	case "photo-album-rename":
		request["albumID"], request["name"] = "synthetic-album-0", inputs[0]
	case "photo-album-delete":
		request["albumID"] = "synthetic-album-0"
	case "photo-album-add":
		request["albumID"], request["photoID"] = "synthetic-album-0", inputs[0]
	case "photo-favorite":
		request["photoID"], request["favorite"] = inputs[0], inputs[1]
	case "photo-delete":
		request["photoID"] = inputs[0]
	default:
		t.Fatal("unknown Source write adapter", operation)
	}
}

func fixtureWriteExpected(t *testing.T, operation string, fixture writeFixture) any {
	t.Helper()
	result := sourceWriteProjection(fixture.Result)
	if operation == "reminder-create" {
		return map[string]any{"reminder": result}
	}
	if operation == "photo-delete" || operation == "photo-album-delete" {
		return map[string]any{"deleted": true}
	}
	if operation == "photo-album-add" {
		return map[string]any{"added": true}
	}
	if operation == "photo-album-create" {
		return map[string]any{"album": result}
	}
	observed, objectPresent := result.(map[string]any)
	if !objectPresent {
		t.Fatal("Source write result is not an object", operation)
	}
	if operation == "photo-album-rename" {
		return map[string]any{"album": observed["album"]}
	}
	if operation == "photo-favorite" {
		photo := observed["photo"].(map[string]any)
		for _, key := range []string{"assetMetadata", "masterMetadata", "versions", "dimensions", "size", "checksum"} {
			delete(photo, key)
		}
		return map[string]any{"photo": photo}
	}
	arguments, ok := observed["arguments"].([]any)
	if !ok || len(arguments) == 0 {
		t.Fatal("Source write result has no mutated argument", operation)
	}
	return expectedReminderWrite(t, operation, observed["value"], arguments)
}

func expectedReminderWrite(t *testing.T, operation string, value any, arguments []any) any {
	t.Helper()
	switch operation {
	case "reminder-update":
		return map[string]any{"reminder": arguments[0]}
	case "reminder-delete":
		reminder := arguments[0].(map[string]any)
		return map[string]any{
			"deleted": true, "modified": reminder["modified"], "recordChangeTag": reminder["recordChangeTag"],
		}
	case "reminder-hashtag-create":
		return map[string]any{"hashtag": value, "reminder": arguments[0]}
	case "reminder-hashtag-update":
		return map[string]any{"hashtag": arguments[0]}
	case "reminder-hashtag-delete":
		return map[string]any{"hashtag": arguments[1], "reminder": arguments[0]}
	case "reminder-recurrence-create":
		return map[string]any{"recurrenceRule": value, "reminder": arguments[0]}
	case "reminder-recurrence-update":
		return map[string]any{"recurrenceRule": arguments[0]}
	case "reminder-recurrence-delete":
		return map[string]any{"recurrenceRule": arguments[1], "reminder": arguments[0]}
	case "reminder-attachment-create":
		return map[string]any{"attachment": value, "reminder": arguments[0]}
	case "reminder-attachment-update":
		return map[string]any{"attachment": arguments[0]}
	case "reminder-attachment-delete":
		return map[string]any{"attachment": arguments[1], "reminder": arguments[0]}
	case "reminder-location-add":
		returned, ok := value.([]any)
		if !ok || len(returned) != 2 {
			t.Fatal("Source location result lost alarm/trigger tuple")
		}
		projectSourceLocationNumbers(t, returned[1].(map[string]any))
		return map[string]any{"alarm": returned[0], "trigger": returned[1], "reminder": arguments[0]}
	default:
		t.Fatal("unknown Source result adapter: " + operation)
	}
	return nil
}

// Source converts location geometry to Python float. The public schema exposes
// float64, whose JSON spelling omits Python's trailing .0. Adapt only those
// schema fields; integer identifiers and exact provider request bytes stay intact.
func projectSourceLocationNumbers(t *testing.T, trigger map[string]any) {
	t.Helper()
	for _, key := range []string{"latitude", "longitude", "radius"} {
		number, ok := trigger[key].(json.Number)
		if !ok {
			t.Fatalf("Source location %s is not numeric", key)
		}
		value, err := number.Float64()
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		trigger[key] = json.Number(encoded)
	}
}

func TestSourceLocationProjectionPreservesNumericValues(t *testing.T) {
	t.Parallel()
	trigger := map[string]any{"latitude": json.Number("1.25"), "longitude": json.Number("2.5"),
		"radius": json.Number("50.0"), "proximity": json.Number("2"), "opaqueInteger": json.Number("9007199254740993")}
	projectSourceLocationNumbers(t, trigger)
	for key, expected := range map[string]json.Number{"latitude": "1.25", "longitude": "2.5", "radius": "50",
		"proximity": "2", "opaqueInteger": "9007199254740993"} {
		if trigger[key] != expected {
			t.Fatalf("%s: want %s, got %v", key, expected, trigger[key])
		}
	}
}
