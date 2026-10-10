package replay_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sourceWriteProjection(t *testing.T, value any) any {
	t.Helper()

	switch node := value.(type) {
	case []any:
		result := make([]any, 0, len(node))
		for _, child := range node {
			result = append(result, sourceWriteProjection(t, child))
		}

		return result
	case map[string]any:
		if text, wrapped := node["$datetime"].(string); wrapped {
			return sourceWriteDate(text)
		}

		if model, wrapped := node["$model"].(string); wrapped {
			result := sourceWriteProjection(t, node["value"])
			if model == "Reminder" {
				defaultWriteReminder(sourceServiceObject(t, result))
			}

			if model == "URLAttachment" {
				attachment := sourceServiceObject(t, result)
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
				result[name] = sourceWriteProjection(t, child)
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
		return testDescriptionKey
	case "fullname":
		return "fullName"
	case "asset":
		return expectedReplayAssetMetadata
	}

	names := map[string]string{
		"list_id": "listID", "reminder_id": expectedReplayReminderID, "parent_reminder_id": expectedParentReminderID,
		"hashtag_ids": testHashtagIDsKey, "attachment_ids": testAttachmentIDsKey,
		"alarm_ids": testAlarmIDsKey, "recurrence_rule_ids": testRecurrenceRuleIDsKey,
		"alarm_id": "alarmID", "alarm_uid": "alarmUID", "trigger_id": "triggerID", "location_uid": "locationUID",
		"file_asset_url":       "fileAssetURL",
		expectedSourceMasterID: testMasterIDKey,
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
	for _, key := range []string{testAlarmIDsKey, testAttachmentIDsKey, testHashtagIDsKey, testRecurrenceRuleIDsKey} {
		if _, exists := value[key]; !exists {
			value[key] = []any{}
		}
	}
}

func writeDateField(key string) bool {
	switch key {
	case "added", "created", testModifiedKey, "completedDate", "dueDate", "startDate":
		return true
	default:
		return false
	}
}

func sourceWriteDate(text string) string {
	instant, err := time.Parse(time.RFC3339Nano, text)
	if err == nil {
		return instant.UTC().Format(time.RFC3339Nano)
	}

	instant, err = time.Parse("2006-01-02T15:04:05", text)
	if err == nil {
		return instant.UTC().Format(time.RFC3339Nano)
	}

	return text
}

func fixtureWriteInput(t *testing.T, operation string, fixture writeFixture) map[string]any {
	t.Helper()

	request := map[string]any{}
	if fixture.Keywords != nil {
		request = sourceServiceObject(t, sourceWriteProjection(t, fixture.Keywords))
	}

	inputs := sourceServiceList(t, sourceWriteProjection(t, fixture.Inputs))

	switch operation {
	case testReminderCreateCommand:
		request["listID"], request["title"] = inputs[0], inputs[1]
		if len(inputs) > 2 {
			request[testDescriptionKey] = inputs[2]
		}
	case testReminderUpdateCommand:
		request[reminderCommand] = inputs[0]
	case testReminderDeleteCommand:
		reminder := sourceServiceObject(t, inputs[0])
		request[expectedReplayReminderID] = reminder["id"]
		request[reminderCLIRevisionField] = reminder[reminderCLIRevisionField]
	case testReminderHashtagCreateCommand:
		request[reminderCommand], request["name"] = inputs[0], inputs[1]
	case testReminderHashtagUpdateCommand:
		request["hashtag"], request["name"] = inputs[0], inputs[1]
	case testReminderHashtagDeleteCommand:
		request[reminderCommand], request["hashtag"] = inputs[0], inputs[1]
	case testReminderRecurrenceCreateCommand:
		request[reminderCommand] = inputs[0]
	case testReminderRecurrenceUpdateCommand:
		request[testRecurrenceRuleKey] = inputs[0]
	case testReminderRecurrenceDeleteCommand:
		request[reminderCommand], request[testRecurrenceRuleKey] = inputs[0], inputs[1]
	case testReminderAttachmentCreateCommand:
		request[reminderCommand], request["url"] = inputs[0], inputs[1]
	case testReminderAttachmentUpdateCommand:
		request[testAttachmentKey] = inputs[0]
	case testReminderAttachmentDeleteCommand:
		request[reminderCommand], request[testAttachmentKey] = inputs[0], inputs[1]
	case testReminderLocationAddCommand:
		request[reminderCommand] = inputs[0]
	default:
		photoWriteInput(t, request, operation, inputs)
	}

	return request
}

func photoWriteInput(t *testing.T, request map[string]any, operation string, inputs []any) {
	t.Helper()

	switch operation {
	case testPhotoAlbumCreateCommand:
		request["name"] = inputs[0]
	case testPhotoAlbumRenameCommand:
		request["albumID"], request["name"] = testSyntheticAlbum0, inputs[0]
	case testPhotoAlbumDeleteCommand:
		request["albumID"] = testSyntheticAlbum0
	case testPhotoAlbumAddCommand:
		request["albumID"], request["photoID"] = testSyntheticAlbum0, inputs[0]
	case testPhotoFavoriteCommand:
		request["photoID"], request["favorite"] = inputs[0], inputs[1]
	case testPhotoDeleteCommand:
		request["photoID"] = inputs[0]
	default:
		t.Fatal("unknown Source write adapter", operation)
	}
}

func fixtureWriteExpected(t *testing.T, operation string, fixture writeFixture) any {
	t.Helper()

	result := sourceWriteProjection(t, fixture.Result)
	if operation == testReminderCreateCommand {
		return map[string]any{reminderCommand: result}
	}

	if operation == testPhotoDeleteCommand || operation == testPhotoAlbumDeleteCommand {
		return map[string]any{"deleted": true}
	}

	if operation == testPhotoAlbumAddCommand {
		return map[string]any{"added": true}
	}

	if operation == testPhotoAlbumCreateCommand {
		return map[string]any{"album": result}
	}

	observed, objectPresent := result.(map[string]any)
	if !objectPresent {
		t.Fatal("Source write result is not an object", operation)
	}

	if operation == testPhotoAlbumRenameCommand {
		return map[string]any{"album": observed["album"]}
	}

	if operation == testPhotoFavoriteCommand {
		photo := sourceServiceObject(t, observed["photo"])
		for _, key := range []string{expectedReplayAssetMetadata, testMasterMetadataKey, testVersionsKey,
			testDimensionsKey, "size", testChecksumKey} {
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
	case testReminderUpdateCommand:
		return map[string]any{reminderCommand: arguments[0]}
	case testReminderDeleteCommand:
		reminder := sourceServiceObject(t, arguments[0])

		return map[string]any{
			"deleted": true, testModifiedKey: reminder[testModifiedKey],
			reminderCLIRevisionField: reminder[reminderCLIRevisionField],
		}
	case testReminderHashtagCreateCommand:
		return map[string]any{"hashtag": value, reminderCommand: arguments[0]}
	case testReminderHashtagUpdateCommand:
		return map[string]any{"hashtag": arguments[0]}
	case testReminderHashtagDeleteCommand:
		return map[string]any{"hashtag": arguments[1], reminderCommand: arguments[0]}
	case testReminderRecurrenceCreateCommand:
		return map[string]any{testRecurrenceRuleKey: value, reminderCommand: arguments[0]}
	case testReminderRecurrenceUpdateCommand:
		return map[string]any{testRecurrenceRuleKey: arguments[0]}
	case testReminderRecurrenceDeleteCommand:
		return map[string]any{testRecurrenceRuleKey: arguments[1], reminderCommand: arguments[0]}
	case testReminderAttachmentCreateCommand:
		return map[string]any{testAttachmentKey: value, reminderCommand: arguments[0]}
	case testReminderAttachmentUpdateCommand:
		return map[string]any{testAttachmentKey: arguments[0]}
	case testReminderAttachmentDeleteCommand:
		return map[string]any{testAttachmentKey: arguments[1], reminderCommand: arguments[0]}
	case testReminderLocationAddCommand:
		returned, ok := value.([]any)
		if !ok || len(returned) != 2 {
			t.Fatal("Source location result lost alarm/trigger tuple")
		}

		projectSourceLocationNumbers(t, sourceServiceObject(t, returned[1]))

		return map[string]any{"alarm": returned[0], "trigger": returned[1], reminderCommand: arguments[0]}
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

	for _, key := range []string{testLatitudeKey, testLongitudeKey, "radius"} {
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

	trigger := map[string]any{testLatitudeKey: json.Number("1.25"), testLongitudeKey: json.Number("2.5"),
		"radius": json.Number("50.0"), testProximityKey: json.Number("2"),
		testOpaqueIntegerKey: json.Number(testOpaqueLargeInteger)}
	projectSourceLocationNumbers(t, trigger)

	for key, expected := range map[string]json.Number{testLatitudeKey: "1.25", testLongitudeKey: "2.5", "radius": "50",
		testProximityKey: "2", testOpaqueIntegerKey: testOpaqueLargeInteger} {
		if trigger[key] != expected {
			t.Fatalf("%s: want %s, got %v", key, expected, trigger[key])
		}
	}
}
