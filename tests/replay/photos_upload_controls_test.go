package replay_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type photoUploadInvalidControl struct {
	fixture string
	body    string
}

func TestPhotoUploadRejectsMalformedNullableContainers(t *testing.T) {
	t.Parallel()

	for name, control := range map[string]photoUploadInvalidControl{
		"reservation-map":   {replayLiteralPhotosUploadReserve1JSON, `{"uploadUrls":null}`},
		"reservation-url":   {replayLiteralPhotosUploadReserve1JSON, `{"uploadUrls":{"synthetic-client-0":null}}`},
		"registration-item": {"photos-upload-register-1.json", `[null]`},
		"status-item":       {"photos-upload-status-1.json", `{"synthetic-job-0":null}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runner, transport := photoUploadControl(t, control.fixture, control.body)
			result, err := runner.call(t)

			var failure *icloud.ClientError

			if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse ||
				!bytes.Equal(failure.ResponseBody(), []byte(control.body)) {
				t.Fatal("malformed upload result did not preserve typed failure and exact body", err)
			}

			checkPhotoUploadControlEvidence(t, runner.scenario, failure)
			checkPhotoUploadControlConsumed(t, transport)
		})
	}
}

func photoUploadControl(t *testing.T, filename, body string) (*photoUploadReplay, *replay.HTTPTransport) {
	t.Helper()
	path := filepath.Join(replayExpectedFixtures, replayExpectedSynthetic, "http", filename)
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)
	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	if body != "" {
		last.Body.Value = marshalFindMyRecovery(t, base64.StdEncoding.EncodeToString([]byte(body)))
	}

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	return newPhotoUploadReplay(t, row, scenario, transport), transport
}

func checkPhotoUploadControlEvidence(t *testing.T, scenario accountScenario, failure *icloud.ClientError) {
	t.Helper()
	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}, last)
	request := scenario.Exchanges[len(scenario.Exchanges)-1].Request
	if failure.CookieScopeURL() != request.Origin+request.Path {
		t.Fatal("malformed upload lost exact cookie scope")
	}

	checkPhotoUploadResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}

func TestPhotoUploadPreservesUnknownNumberPrecision(t *testing.T) {
	t.Parallel()
	const number = photoUploadOpaqueInteger
	const body = `{"singleFile":{"referenceChecksum":"synthetic-reference","size":3,` +
		`"fileChecksum":"synthetic-checksum","wrappingKey":"synthetic-key",` +
		`"receipt":"synthetic-receipt","opaque":` + number + `}}`
	runner, transport := photoUploadControl(t, replayLiteralPhotosUploadBytesBinaryJSON, body)
	result, err := runner.call(t)
	if err != nil {
		t.Fatal(err)
	}

	receipt, ok := result.value.(icloud.PhotoUploadReceipt)
	if !ok || string(receipt.AdditionalProperties["opaque"]) != number {
		t.Fatal("unknown upload JSON number lost exact precision")
	}

	checkPhotoUploadControlConsumed(t, transport)
}

type photoUploadOwnedReader struct {
	*bytes.Reader

	closed bool
}

func (reader *photoUploadOwnedReader) Close() error {
	reader.closed = true

	return nil
}

func TestPhotoUploadPreservesCallerCursorAndReaderOwnership(t *testing.T) {
	t.Parallel()
	runner, transport := photoUploadControl(t, replayLiteralPhotosUploadBytesBinaryJSON, "")
	_, content, _ := photoUploadContent(t, runner.row)
	data, err := io.ReadAll(content)
	if err != nil {
		t.Fatal(err)
	}

	const ignored = photoControlCursorPrefix
	reader := &photoUploadOwnedReader{Reader: bytes.NewReader(append([]byte(ignored), data...)), closed: false}
	_, err = reader.Seek(int64(len(ignored)), io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}

	var inputs []string

	authReplayDecode(t, runner.row["inputs"], &inputs)
	result, err := runner.client.SendPhotoUploadBytes(t.Context(), icloud.SendPhotoUploadBytesRequest{
		Auth: runner.auth, Library: nil, URL: inputs[0], Content: reader})
	if err != nil || result == nil || reader.closed {
		t.Fatal("upload closed caller-owned reader or ignored its cursor", err)
	}

	position, err := reader.Seek(0, io.SeekCurrent)
	if err != nil || position != int64(len(ignored)+len(data)) {
		t.Fatal("upload did not consume the declared cursor range", position, err)
	}

	checkPhotoUploadControlConsumed(t, transport)
}

func checkPhotoUploadControlConsumed(t *testing.T, transport *replay.HTTPTransport) {
	t.Helper()
	err := transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

var _ io.ReadSeekCloser = (*photoUploadOwnedReader)(nil)
