package replay_test

import (
	"encoding/json"
	"testing"
)

const lookupFixturePrefix = "get"
const lookupResultField = "result"

func photoCLIArgs(t *testing.T, operation string, row map[string]json.RawMessage) []string {
	t.Helper()

	args := []string{}

	if (operation == photoCountCommand || operation == photoAssetsCommand || operation == photoLookupCommand) &&
		len(row["album"]) != 0 {
		var album string

		decode(t, row["album"], &album)
		args = append(args, "--album", album)
	}

	if operation == photoLookupCommand {
		var inputs []string

		decode(t, row["inputs"], &inputs)
		args = append(args, "--photo", inputs[0])
	}

	return args
}

func checkPhotoLookupCLI(t *testing.T, row, actual map[string]json.RawMessage) {
	t.Helper()

	if len(actual) != 1 || len(actual["photo"]) == 0 {
		t.Fatal("photo lookup CLI exposed unexpected fields")
	}

	var expected any

	decode(t, row[lookupResultField], &expected)

	if expected == nil {
		checkReminderCLIValue(t, actual["photo"], nil)

		return
	}

	// Reuse the complete Source photo projection, including resources and opaque metadata.
	expectedArray := append(append(json.RawMessage("["), row[lookupResultField]...), ']')
	actualArray := append(append(json.RawMessage("["), actual["photo"]...), ']')
	checkPhotoAssetsCLI(t, map[string]json.RawMessage{lookupResultField: expectedArray},
		map[string]json.RawMessage{"photos": actualArray})
}
