package replay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const photoStatusCommand = "photos-status"
const photoAlbumsCommand = "photo-albums"
const photoCountCommand = "photo-count"
const photoAssetsCommand = "photo-assets"
const photoLookupCommand = "photo"

type photoPrivacyClient struct {
	icloud.Client

	result *icloud.ListPhotoAssetsResult
}

func (client *photoPrivacyClient) ListPhotoAssets(
	context.Context, icloud.ListPhotoAssetsRequest,
) (*icloud.ListPhotoAssetsResult, error) {
	return client.result, nil
}

// This synthetic SDK boundary control complements the complete paired Source
// scenarios below; its opaque values are not a provider capture.
func TestPhotoReadOpaquePrivacy(t *testing.T) {
	t.Parallel()
	var result icloud.ListPhotoAssetsResult
	decode(t, json.RawMessage(`{"photos":[{"id":"public-id","masterID":"public-master","filename":"public.jpg","itemType":"image","isLivePhoto":false,"created":"2020-01-01T00:00:00Z","added":"2020-01-02T00:00:00Z","assetMetadata":{"nested":[{"url":"https://private.example.invalid/asset?token=asset-secret","opaque":{"headers":["metadata-secret"]}}]},"versions":{"original":{"url":{"nested":["https://private.example.invalid/download?token=resource-secret"]},"checksum":{"opaque":"checksum-secret"},"size":42,"type":"public.jpeg"}},"dimensions":[{"opaque":"dimension-secret"},null],"size":{"opaque":["size-secret"]}}],"responses":[{"statusCode":200,"headers":[{"name":"Set-Cookie","value":"session=receipt-secret"}],"cookieScopeURL":"https://private.example.invalid/photos"}]}`), &result)
	original := reminderCLIEncode(t, result)
	directory := t.TempDir()
	session, saved := filepath.Join(directory, "session.json"), filepath.Join(directory, "result.json")
	var auth icloud.AuthContext

	auth.AccountID, auth.ClientID = "synthetic-account", "synthetic-client"
	auth.PhotosServiceURL = "https://photos.example.invalid"
	auth.Headers = []icloud.Header{}
	writeSyncJSON(t, session, auth)
	var output, diagnostic bytes.Buffer
	err := command.Run(t.Context(), &photoPrivacyClient{Client: nil, result: &result},
		[]string{sessionFlag, session, "--save-result", saved, photoAssetsCommand}, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.Len() != 0 {
		t.Fatal("photo privacy control printed diagnostics")
	}
	private, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	checkReminderCLIValue(t, private, result)
	var expected map[string]json.RawMessage
	decode(t, json.RawMessage(`{"photos":[{"id":"public-id","masterID":"public-master","filename":"public.jpg","itemType":"image","isLivePhoto":false,"created":"2020-01-01T00:00:00Z","added":"2020-01-02T00:00:00Z"}]}`), &expected)
	checkReminderCLIValue(t, output.Bytes(), expected)
	for _, marker := range []string{
		"asset-secret", "metadata-secret", "resource-secret", "checksum-secret", "dimension-secret",
		"size-secret", "receipt-secret",
	} {
		if !bytes.Contains(private, []byte(marker)) || strings.Contains(output.String(), marker) {
			t.Fatal("opaque private value was lost from the private result or exposed on stdout")
		}
	}
	if !bytes.Equal(original, reminderCLIEncode(t, result)) {
		t.Fatal("console projection mutated the caller-owned SDK result")
	}
}

func TestPhotoReadCommands(t *testing.T) {
	t.Parallel()

	for prefix, operation := range map[string]string{"index": photoStatusCommand,
		"albums": photoAlbumsCommand, "count": photoCountCommand, "assets": photoAssetsCommand,
		lookupFixturePrefix: photoLookupCommand} {
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

		if prefix == lookupFixturePrefix {
			want = 22
		}

		if len(paths) != want {
			t.Fatal("photo CLI scenario inventory changed")
		}

		for _, path := range paths {
			t.Run(filepath.Base(path), func(t *testing.T) {
				t.Parallel()
				runPhotoReadScenario(t, operation, readObject(t, path))
			})
		}
	}
}

func runPhotoReadScenario(t *testing.T, operation string, row map[string]json.RawMessage) {
	t.Helper()
	var exchanges []replay.Exchange
	decode(t, row["exchanges"], &exchanges)
	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	session, saved := filepath.Join(directory, "session.json"), filepath.Join(directory, "result.json")
	writeSyncJSON(t, session, fixtureAuth(t, row["initial_state"]))
	options := photoCLIArgs(t, operation, row)
	args := make([]string, 0, 5+len(options))
	args = append(args, sessionFlag, session, "--save-result", saved)
	args = append(args, options...)
	args = append(args, operation)
	var output, diagnostic bytes.Buffer
	err = command.Run(t.Context(), client, args, &output, &diagnostic)
	if len(row["error"]) != 0 {
		checkPhotoCLIFailure(t, row, output.Bytes(), err)
		if _, readErr := os.ReadFile(saved); !errors.Is(readErr, os.ErrNotExist) {
			t.Fatal("failed photo read wrote a partial private result")
		}
	} else {
		if err != nil {
			t.Fatal(err)
		}
		checkPhotoReadPrivateSource(t, operation, row, saved, output.Bytes(), exchanges)
	}
	if diagnostic.Len() != 0 {
		t.Fatal("photo read printed unexpected diagnostics")
	}
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
}

func checkPhotoReadPrivateSource(t *testing.T, operation string, row map[string]json.RawMessage,
	saved string, output []byte, exchanges []replay.Exchange,
) {
	t.Helper()
	data, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	var complete map[string]json.RawMessage
	decode(t, data, &complete)
	if operation == photoStatusCommand {
		checkReminderCLIValue(t, complete["metadata"], referenceResponseMetadata(exchanges[len(exchanges)-1]))
		delete(complete, "metadata")
	} else {
		metadata := make([]icloud.ResponseMetadata, 0, len(exchanges))
		for _, exchange := range exchanges {
			metadata = append(metadata, referenceResponseMetadata(exchange))
		}
		checkReminderCLIValue(t, complete["responses"], metadata)
		delete(complete, "responses")
	}
	// Compare every business field, including URLs and opaque asset/resource
	// values, with pinned Source before applying the explicit console omissions.
	checkPhotoCLIOutcome(t, operation, row, reminderCLIEncode(t, complete), nil)
	checkReminderCLIValue(t, output, photoReadConsoleProjection(t, operation, complete))
}

func photoReadConsoleProjection(t *testing.T, operation string,
	complete map[string]json.RawMessage,
) map[string]json.RawMessage {
	t.Helper()
	result := maps.Clone(complete)
	switch operation {
	case photoStatusCommand:
		delete(result, "syncToken")
	case photoAssetsCommand:
		var photos []map[string]json.RawMessage
		decode(t, result["photos"], &photos)
		for _, photo := range photos {
			deletePhotoConsoleContainers(photo)
		}
		result["photos"] = reminderCLIEncode(t, photos)
	case photoLookupCommand:
		if !bytes.Equal(bytes.TrimSpace(result["photo"]), []byte("null")) {
			var photo map[string]json.RawMessage
			decode(t, result["photo"], &photo)
			deletePhotoConsoleContainers(photo)
			result["photo"] = reminderCLIEncode(t, photo)
		}
	}
	return result
}

func deletePhotoConsoleContainers(photo map[string]json.RawMessage) {
	// These four generated photo containers are the privacy contract. All
	// remaining scalar semantic fields must survive unchanged; no recursive
	// production sanitizer or arbitrary field selection is used as an oracle.
	for _, name := range []string{"assetMetadata", "versions", "dimensions", "size"} {
		delete(photo, name)
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

	if operation == photoLookupCommand {
		checkPhotoLookupCLI(t, row, actual)

		return
	}

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
