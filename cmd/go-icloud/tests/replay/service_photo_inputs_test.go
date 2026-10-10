package replay_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestPhotoTypedInputsPairedReferenceReplay(t *testing.T) {
	t.Parallel()

	for _, scenario := range []writeReplayCase{
		{photoStatusCommand, "photos-index-ready"},
		{photoAlbumsCommand, "photos-albums-1"},
		{photoCountCommand, "photos-count-1"},
		{photoAssetsCommand, expectedOneAssetFixture},
		{photoLookupCommand, "photos-get-existing"},
	} {
		t.Run(scenario.fixture, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(writeFixtureDirectory, scenario.fixture+".json")
			runPhotoReadInputScenario(t, scenario.operation, readObject(t, path), true)
		})
	}
}

func typedPhotoFixtureInput(t *testing.T, operation string, row map[string]json.RawMessage) map[string]any {
	t.Helper()

	input := map[string]any{"auth": map[string]any{expectedClientID: "foreign"}}

	if operation == photoCountCommand || operation == photoAssetsCommand || operation == photoLookupCommand {
		album := "Library"
		if value := row["album"]; len(value) != 0 {
			decode(t, value, &album)
		}

		input["album"] = album
	}

	if operation == photoLookupCommand {
		var values []string

		decode(t, row["inputs"], &values)
		input["photoID"] = values[0]
	}

	return input
}
