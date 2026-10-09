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

const photoDownloadCommand = "photo-download"

func TestPhotoDownloadCommand(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("../../../../tests/replay/fixtures/synthetic/http/photos-download-*.json")
	if err != nil || len(paths) != 25 {
		t.Fatal("photo download CLI inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runReminderCommand(t, photoDownloadCommand, readObject(t, path))
		})
	}
}

func photoDownloadVersionArgs(t *testing.T, row map[string]json.RawMessage) []string {
	t.Helper()

	if len(row["version"]) == 0 {
		return nil
	}

	var version string

	decode(t, row["version"], &version)

	return []string{"--version", version}
}

func checkPhotoDownloadCLI(t *testing.T, row map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	if len(row["error"]) != 0 {
		checkPhotoDownloadCLIFailure(t, row, output, err)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var actual map[string]json.RawMessage

	decode(t, output, &actual)

	if len(actual) != 1 || len(actual["content"]) == 0 {
		t.Fatal("photo download CLI exposed unexpected fields")
	}
	// Source returns base64 bytes, including the distinct empty and unavailable values.
	checkReminderCLIValue(t, actual["content"], row["result"])
}

func checkPhotoDownloadCLIFailure(t *testing.T, row map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	var failure *icloud.ClientError
	if len(output) != 0 || !errors.As(err, &failure) {
		t.Fatal("photo download lost typed error or printed partial bytes")
	}

	var exchanges []replay.Exchange

	decode(t, row["exchanges"], &exchanges)

	last := exchanges[len(exchanges)-1]
	if last.Error != "" {
		if failure.Kind() != icloud.Transport || failure.StatusCode() != 0 ||
			len(failure.ResponseBody()) != 0 || len(failure.ResponseHeaders()) != 0 || failure.CookieScopeURL() != "" {
			t.Fatal("photo download fabricated transport failure response evidence")
		}
	} else {
		checkPhotoDownloadCLIProvider(t, failure, last)
	}

	checkReminderCLIPrior(t, row, failure)
}

func checkPhotoDownloadCLIProvider(t *testing.T, failure *icloud.ClientError, last replay.Exchange) {
	t.Helper()

	kind := expectedFailureKind(last.Response.Status)
	if last.Response.Status == 404 {
		kind = icloud.NotFound
	}

	if failure.Kind() != kind || failure.StatusCode() != last.Response.Status ||
		!bytes.Equal(failure.ResponseBody(), referenceResponseBody(t, last.Response.Body)) {
		t.Fatal("photo download changed provider failure class, status or exact bytes")
	}

	metadata := icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}
	if !reflect.DeepEqual(metadata, referenceResponseMetadata(last)) {
		t.Fatal("photo download lost complete failure metadata")
	}
}
