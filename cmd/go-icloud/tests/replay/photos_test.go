package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const photoStatusCommand = "photos-status"
const photoAlbumsCommand = "photo-albums"
const photoCountCommand = "photo-count"
const photoAssetsCommand = "photo-assets"

func TestPhotoReadCommands(t *testing.T) {
	t.Parallel()

	for prefix, operation := range map[string]string{"index": photoStatusCommand,
		"albums": photoAlbumsCommand, "count": photoCountCommand, "assets": photoAssetsCommand} {
		paths, err := filepath.Glob("../../../../tests/replay/fixtures/synthetic/http/photos-" + prefix + "-*.json")
		if err != nil {
			t.Fatal(err)
		}

		want := 27
		if prefix == "albums" {
			want = 41
		}

		if prefix == "count" {
			want = 56
		}

		if prefix == "assets" {
			want = 109
		}

		if len(paths) != want {
			t.Fatal("photo CLI scenario inventory changed")
		}

		for _, path := range paths {
			t.Run(filepath.Base(path), func(t *testing.T) {
				t.Parallel()
				runReminderCommand(t, operation, readObject(t, path))
			})
		}
	}
}

func checkPhotoCLIOutcome(t *testing.T, operation string, row map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	if len(row["error"]) != 0 {
		checkPhotoCLIFailure(t, row, output, err)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var actual map[string]json.RawMessage

	decode(t, output, &actual)

	if operation == photoAssetsCommand {
		checkPhotoAssetsCLI(t, row, actual)

		return
	}

	if operation == photoCountCommand {
		if len(actual) != 1 {
			t.Fatal("photo count CLI exposed unexpected fields")
		}

		var expected int64

		decode(t, row["result"], &expected)
		checkReminderCLIValue(t, actual["count"], expected)

		return
	}

	if operation == photoStatusCommand {
		var expected map[string]json.RawMessage

		decode(t, row["result"], &expected)
		expected["syncToken"] = expected["sync_token"]
		delete(expected, "sync_token")
		checkReminderCLIValue(t, output, expected)

		return
	}

	checkPhotoAlbumsCLI(t, row, actual)
}

func checkPhotoAlbumsCLI(t *testing.T, row, actual map[string]json.RawMessage) {
	t.Helper()

	if len(actual) != 1 {
		t.Fatal("photo CLI exposed unexpected fields")
	}

	var expected []map[string]json.RawMessage

	decode(t, row["result"], &expected)

	for _, album := range expected {
		album["fullName"] = album["fullname"]
		delete(album, "fullname")
		album["recordChangeTag"] = album["record_change_tag"]
		delete(album, "record_change_tag")
	}

	checkReminderCLIValue(t, actual["albums"], expected)
}

func checkPhotoCLIFailure(t *testing.T, row map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	var failure *icloud.ClientError
	if len(output) != 0 || !errors.As(err, &failure) {
		t.Fatal("photo CLI lost error or printed partial result")
	}

	var (
		exchanges   []replay.Exchange
		sourceError map[string]string
	)

	decode(t, row["exchanges"], &exchanges)
	decode(t, row["error"], &sourceError)

	last := exchanges[len(exchanges)-1]

	kind := expectedFailureKind(last.Response.Status)
	if last.Response.Status < 400 {
		kind = icloud.InvalidResponse
	}

	if sourceError["type"] == "PyiCloudServiceNotActivatedException" {
		kind = icloud.Unavailable
	}

	if sourceError["type"] == "KeyError" {
		kind = icloud.NotFound
	}

	if failure.Kind() != kind || failure.StatusCode() != last.Response.Status ||
		!bytes.Equal(failure.ResponseBody(), referenceResponseBody(t, last.Response.Body)) {
		t.Fatal("photo CLI changed failure class, status or body")
	}

	metadata := icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}
	if !reflect.DeepEqual(metadata, referenceResponseMetadata(last)) {
		t.Fatal("photo CLI lost failure metadata")
	}

	checkReminderCLIPrior(t, row, failure)
}
