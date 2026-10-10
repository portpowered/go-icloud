package command_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoUploadCommandsPairedReferenceReplay(t *testing.T) {
	t.Parallel()
	for _, scenario := range []writeReplayCase{
		{"photo-upload-reserve", "photos-upload-reserve-1"},
		{"photo-upload-send", "photos-upload-bytes-binary"},
		{"photo-upload-register", "photos-upload-register-1"},
		{"photo-upload-file", "photos-upload-pipeline-success"},
		{"photo-upload", "photos-upload-service-album"},
	} {
		t.Run(scenario.fixture, func(t *testing.T) { t.Parallel(); runUploadWriteReplay(t, scenario) })
	}
}

func runUploadWriteReplay(t *testing.T, scenario writeReplayCase) {
	t.Helper()
	fixture := loadWriteFixture(t, scenario.fixture)
	transport, err := replay.NewHTTPTransport(fixture.Exchanges)
	if err != nil {
		t.Fatal(err)
	}
	client := fixtureWriteClient(t, fixture, transport)
	directory := t.TempDir()
	session := filepath.Join(directory, "session.json")
	request := filepath.Join(directory, "request.json")
	result := filepath.Join(directory, "result.json")
	writeFixtureValue(t, session, fixtureWriteAuthentication(t, fixture))
	writeFixtureValue(t, request, uploadWriteRequest(t, scenario.operation, fixture))
	args := []string{"--session", session, "--request", request, "--save-result", result}
	if len(fixture.File) != 0 {
		content := filepath.Join(directory, "content")
		var encoded string
		decodeWriteFixture(t, fixture.File["body"], &encoded)
		data, decodeErr := base64.StdEncoding.DecodeString(encoded)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		writeProbeFile(t, content, data)
		args = append(args, "--file", content)
	}

	args = append(args, scenario.operation)
	var output, diagnostic bytes.Buffer
	err = command.Run(t.Context(), client, args, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
	checkUploadWriteResult(t, scenario.operation, fixture, result, output.Bytes())
}

func uploadWriteRequest(t *testing.T, operation string, fixture writeFixture) map[string]any {
	t.Helper()
	switch operation {
	case "photo-upload-reserve":
		return map[string]any{"assets": fixture.Keywords["assets"]}
	case "photo-upload-send":
		return map[string]any{"url": fixture.Inputs[0]}
	case "photo-upload-register":
		return uploadRegisterWriteRequest(t, fixture)
	default:
		var filename string
		var seconds int64
		decodeWriteFixture(t, fixture.File["name"], &filename)
		decodeWriteFixture(t, fixture.File["modified_seconds"], &seconds)
		request := map[string]any{
			"filename": filename, "modificationTime": time.Unix(seconds, 0).UTC().Format(time.RFC3339Nano),
			"localTimeZoneID": "UTC", "timeZoneOffset": 0}
		if operation == "photo-upload" {
			request["hydrate"] = true
			request["albumID"] = fixture.Keywords["album"]
		}
		return request
	}
}

func uploadRegisterWriteRequest(t *testing.T, fixture writeFixture) map[string]any {
	t.Helper()
	source, ok := fixture.Keywords["files"].([]any)
	if !ok {
		t.Fatal("Source register inputs lost file list")
	}
	files := make([]any, 0, len(source))
	for _, item := range source {
		file := item.(map[string]any)
		milliseconds := file["lastModDate"].(json.Number)
		seconds, err := milliseconds.Int64()
		if err != nil {
			t.Fatal(err)
		}

		files = append(files, map[string]any{"filename": file["fileName"],
			"modificationTime": time.UnixMilli(seconds).UTC().Format(time.RFC3339Nano),
			"timeZoneOffset":   file["timeZoneOffset"], "receipt": file["singleFileUploadRequest"]})
	}
	return map[string]any{
		"files": files, "importGroup": fixture.Keywords["import_group"],
		"localTimeZoneID": fixture.Keywords["local_time_zone_id"],
	}
}

func checkUploadWriteResult(t *testing.T, operation string, fixture writeFixture, path string, output []byte) {
	t.Helper()
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]json.RawMessage
	decodeWriteFixture(t, saved, &result)
	var responses []icloud.ResponseMetadata
	decodeWriteFixture(t, result["responses"], &responses)
	checkWriteResponses(t, responses, fixture.Exchanges)
	var console any
	decodeWriteFixture(t, output, &console)
	var expected any

	switch operation {
	case "photo-upload-reserve":
		compareUploadPrivateValue(t, result["uploadURLs"], fixture.Result)
		expected = map[string]any{}
	case "photo-upload-send":
		compareUploadPrivateValue(t, result["receipt"], fixture.Result)
		expected = map[string]any{}
	case "photo-upload-register":
		registrations := fixture.Result.([]any)
		expected = map[string]any{"registrations": sourceUploadRegistrations(t, registrations)}
	default:
		observed := fixture.Result.(map[string]any)
		acknowledgement := observed["value"]
		if operation == "photo-upload" {
			acknowledgement = uploadedRegistrationReply(t, fixture)
		}
		registration := sourceUploadRegistration(t, acknowledgement.(map[string]any))
		expected = map[string]any{"registration": registration}
		if operation == "photo-upload" {
			photo := sourceWriteProjection(observed).(map[string]any)
			for _, key := range []string{"assetMetadata", "masterMetadata", "versions", "dimensions", "size", "checksum"} {
				delete(photo, key)
			}
			expected = map[string]any{"registration": registration, "indexed": true, "photo": photo}
		}
	}
	if !reflect.DeepEqual(console, expected) {
		t.Fatalf("Source upload console projection differs\nwant %#v\ngot %#v", expected, console)
	}
	if strings.Contains(string(output), "synthetic-token") || strings.Contains(string(output), "synthetic-receipt") ||
		strings.Contains(string(output), "synthetic-key") {
		t.Fatal("upload secrets leaked into console")
	}
}

func uploadedRegistrationReply(t *testing.T, fixture writeFixture) any {
	t.Helper()
	for _, exchange := range fixture.Exchanges {
		if !strings.HasSuffix(exchange.Request.Path, "/putAsset") {
			continue
		}
		var encoded string
		decodeWriteFixture(t, exchange.Response.Body.Value, &encoded)
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		var records []any
		decodeWriteFixture(t, data, &records)
		if len(records) != 1 {
			t.Fatal("upload acknowledgement lost record")
		}
		return records[0]
	}
	t.Fatal("upload registration reply missing")
	return nil
}

func compareUploadPrivateValue(t *testing.T, data []byte, expected any) {
	t.Helper()
	var value any
	decodeWriteFixture(t, data, &value)
	if !reflect.DeepEqual(value, expected) {
		t.Fatal("private upload result lost complete Source receipt")
	}
}

func sourceUploadRegistrations(t *testing.T, source []any) []any {
	t.Helper()
	result := make([]any, 0, len(source))
	for _, item := range source {
		registration := item.(map[string]any)
		result = append(result, sourceUploadRegistration(t, registration))
	}
	return result
}

func sourceUploadRegistration(t *testing.T, source map[string]any) map[string]any {
	t.Helper()
	var status any
	if response, ok := source["response"].(map[string]any); ok {
		status = map[string]any{"status": response["status"], "retryable": response["isRetryable"]}
	}
	return map[string]any{"jobID": source["uploadJobId"], "masterID": source["cplMaster"], "photoID": source["cplAsset"],
		"duplicate": false, "status": status}
}
