package command_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestTypedPhotoReadsPairedReferenceReplay(t *testing.T) {
	t.Parallel()

	for _, scenario := range []writeReplayCase{
		{"photo-libraries", "photos-libraries-empty"},
		{"photo-cursor", "photos-sync-cached"},
		{"photo-changes", "photos-changes-empty"},
		{"photo-library-changes", "photos-container-private-changes-0"},
		{"photos-recently-added", "photos-recently-added-one"},
		{"shared-photo-albums", "photos-upload-shared-albums-0"},
		{"shared-photo-count", "photos-upload-shared-count-1"},
		{"shared-photos", "photos-upload-shared-assets-1"},
		{"shared-photo", "photos-upload-shared-get-missing"},
		{"shared-photo-download", "photos-upload-shared-download-binary"},
		{"photo-upload-status", "photos-upload-status-1"},
	} {
		t.Run(scenario.fixture, func(t *testing.T) { t.Parallel(); runTypedReadReplay(t, scenario) })
	}
}

func runTypedReadReplay(t *testing.T, scenario writeReplayCase) {
	t.Helper()
	fixture := loadWriteFixture(t, scenario.fixture)
	transport, err := replay.NewHTTPTransport(fixture.Exchanges)
	if err != nil {
		t.Fatal(err)
	}
	client := fixtureWriteClient(t, fixture, transport)
	auth := fixtureWriteAuthentication(t, fixture)
	if origin := fixture.Initial["shared_streams_origin"]; len(origin) != 0 {
		decodeWriteFixture(t, origin, &auth.SharedPhotosServiceURL)
	}
	directory := t.TempDir()
	session := filepath.Join(directory, "session.json")
	request, result := filepath.Join(directory, "request.json"), filepath.Join(directory, "result.json")
	writeFixtureValue(t, session, auth)
	input := typedReadFixtureRequest(scenario.operation, fixture)
	input["auth"] = map[string]any{"accountID": "foreign", "photosServiceURL": "https://foreign.example.invalid"}
	writeFixtureValue(t, request, input)
	var output, diagnostic bytes.Buffer
	args := []string{"--session", session, "--request", request, "--save-result", result, scenario.operation}
	err = command.Run(t.Context(), client, args, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
	private, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	var original map[string]json.RawMessage
	var actual map[string]any

	decodeWriteFixture(t, private, &original)
	decodeWriteFixture(t, output.Bytes(), &actual)
	var responses []icloud.ResponseMetadata

	decodeWriteFixture(t, original["responses"], &responses)
	checkWriteResponses(t, responses, fixture.Exchanges)
	if scenario.operation == "photo-libraries" {
		libraries, listPresent := actual["libraries"].([]any)
		if !listPresent || len(libraries) != 1 {
			t.Fatal("root inventory differs")
		}
		library, objectPresent := libraries[0].(map[string]any)
		if !objectPresent || library["id"] != "root" {
			t.Fatal("root inventory differs")
		}
		return
	}
	expected := typedReadFixtureExpected(t, scenario.operation, fixture)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("console projection differs\nactual: %#v\nSource: %#v", actual, expected)
	}
	for _, secret := range []string{
		"https://assets.example.invalid", "synthetic-next", "synthetic-cached", "synthetic-account",
	} {
		if strings.Contains(output.String(), secret) {
			t.Fatal("private provider state reached ordinary output")
		}
	}
}

func typedReadFixtureRequest(operation string, fixture writeFixture) map[string]any {
	switch operation {
	case "photo-changes":
		return map[string]any{"since": fixture.Keywords["since"]}
	case "shared-photo-count", "shared-photos":
		return map[string]any{"album": "synthetic-stream-0"}
	case "shared-photo", "shared-photo-download":
		return map[string]any{"album": "synthetic-stream-0", "photoID": fixture.Inputs[0]}
	case "photo-upload-status":
		return map[string]any{"jobIDs": fixture.Inputs[0]}
	default:
		return map[string]any{}
	}
}

func typedReadFixtureExpected(t *testing.T, operation string, fixture writeFixture) map[string]any {
	t.Helper()
	switch operation {
	case "photo-cursor":
		return map[string]any{}
	case "photo-changes":
		return map[string]any{"changes": []any{}}
	case "photo-library-changes":
		return map[string]any{"zones": []any{}, "moreComing": false}
	case "shared-photo-albums":
		return map[string]any{"albums": []any{}}
	case "shared-photo-count":
		return map[string]any{"count": fixture.Result}
	case "shared-photo":
		return map[string]any{"photo": nil}
	case "shared-photo-download":
		return map[string]any{"content": fixture.Result}
	case "photos-recently-added", "shared-photos":
		return map[string]any{"photos": typedReadSourcePhotos(t, fixture.Result, operation == "shared-photos")}
	case "photo-upload-status":
		jobs := make(map[string]any)
		source, objectPresent := fixture.Result.(map[string]any)
		if !objectPresent {
			t.Fatal("Source upload status is not an object")
		}

		for id, raw := range source {
			job := raw.(map[string]any)
			value := sourceWriteProjection(job["value"]).(map[string]any)
			value["unknown"] = job["is_unknown"]
			jobs[id] = value
		}
		return map[string]any{"jobs": jobs}
	default:
		t.Fatal("missing Source read projection", operation)
		return nil
	}
}

func typedReadSourcePhotos(t *testing.T, source any, shared bool) []any {
	t.Helper()
	photos := sourceWriteProjection(source).([]any)
	for index, raw := range photos {
		photo := raw.(map[string]any)
		for _, key := range []string{"assetMetadata", "masterMetadata", "versions", "dimensions", "size", "checksum"} {
			delete(photo, key)
		}
		if _, exists := photo["isLivePhoto"]; !exists {
			photo["isLivePhoto"] = false
		}
		if shared {
			likeCount, liked := photo["likeCount"], photo["liked"]
			delete(photo, "likeCount")
			delete(photo, "liked")
			photos[index] = map[string]any{"photo": photo, "likeCount": likeCount, "liked": liked}
		}
	}
	return photos
}
