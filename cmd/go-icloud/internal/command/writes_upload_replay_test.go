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
		{testPhotoUploadReserveCommand, "photos-upload-reserve-1"},
		{testPhotoUploadSendCommand, "photos-upload-bytes-binary"},
		{testPhotoUploadRegisterCommand, "photos-upload-register-1"},
		{testPhotoUploadFileCommand, "photos-upload-pipeline-success"},
		{testPhotoUploadCommand, "photos-upload-service-album"},
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
	session := filepath.Join(directory, testSessionJsonFilename)
	request := filepath.Join(directory, testRequestJsonFilename)
	result := filepath.Join(directory, testResultJsonFilename)
	writeFixtureValue(t, session, fixtureWriteAuthentication(t, fixture))
	writeFixtureValue(t, request, uploadWriteRequest(t, scenario.operation, fixture))
	args := []string{testSessionFlag, session, testRequestFlag, request, testSaveResultFlag, result}
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
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
	checkUploadWriteResult(t, scenario.operation, fixture, result, output.Bytes())
}

func uploadWriteRequest(t *testing.T, operation string, fixture writeFixture) map[string]any {
	t.Helper()
	switch operation {
	case testPhotoUploadReserveCommand:
		return map[string]any{"assets": fixture.Keywords["assets"]}
	case testPhotoUploadSendCommand:
		return map[string]any{"url": fixture.Inputs[0]}
	case testPhotoUploadRegisterCommand:
		return uploadRegisterWriteRequest(t, fixture)
	default:

		var filename string

		var seconds int64

		decodeWriteFixture(t, fixture.File["name"], &filename)
		decodeWriteFixture(t, fixture.File["modified_seconds"], &seconds)
		request := map[string]any{
			testFilenameKey: filename, testModificationTimeKey: time.Unix(seconds, 0).UTC().Format(time.RFC3339Nano),
			testLocalTimeZoneIDKey: "UTC", testTimeZoneOffsetKey: 0}
		if operation == testPhotoUploadCommand {
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
		file := sourceServiceObject(t, item)
		milliseconds := sourceServiceNumber(t, file["lastModDate"])
		seconds, err := milliseconds.Int64()
		if err != nil {
			t.Fatal(err)
		}

		files = append(files, map[string]any{testFilenameKey: file["fileName"],
			testModificationTimeKey: time.UnixMilli(seconds).UTC().Format(time.RFC3339Nano),
			testTimeZoneOffsetKey:   file[testTimeZoneOffsetKey], "receipt": file["singleFileUploadRequest"]})
	}
	return map[string]any{
		"files": files, "importGroup": fixture.Keywords["import_group"],
		testLocalTimeZoneIDKey: fixture.Keywords["local_time_zone_id"],
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

	decodeWriteFixture(t, result[testResponsesKey], &responses)
	checkWriteResponses(t, responses, fixture.Exchanges)

	var console any

	decodeWriteFixture(t, output, &console)

	var expected any

	switch operation {
	case testPhotoUploadReserveCommand:
		compareUploadPrivateValue(t, result[testUploadURLsKey], fixture.Result)
		expected = map[string]any{}
	case testPhotoUploadSendCommand:
		compareUploadPrivateValue(t, result["receipt"], fixture.Result)
		expected = map[string]any{}
	case testPhotoUploadRegisterCommand:
		registrations := sourceServiceList(t, fixture.Result)
		expected = map[string]any{"registrations": sourceUploadRegistrations(t, registrations)}
	default:
		observed := sourceServiceObject(t, fixture.Result)
		acknowledgement := observed["value"]
		if operation == testPhotoUploadCommand {
			acknowledgement = uploadedRegistrationReply(t, fixture)
		}
		registration := sourceUploadRegistration(t, sourceServiceObject(t, acknowledgement))
		expected = map[string]any{testRegistrationKey: registration}
		if operation == testPhotoUploadCommand {
			photo := sourceServiceObject(t, sourceWriteProjection(t, observed))
			for _, key := range []string{testAssetMetadataKey, testMasterMetadataKey, testVersionsKey,
				testDimensionsKey, "size", testChecksumKey} {
				delete(photo, key)
			}
			expected = map[string]any{testRegistrationKey: registration, "indexed": true, "photo": photo}
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
		registration := sourceServiceObject(t, item)
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
	return map[string]any{"jobID": source["uploadJobId"], testMasterIDKey: source["cplMaster"],
		"photoID":        source["cplAsset"],
		testDuplicateKey: false, "status": status}
}
