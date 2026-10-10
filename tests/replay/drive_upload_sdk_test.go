package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type driveUploadScenario struct {
	driveReadScenario

	//nolint:tagliatelle // LIB-05: portable initial-input spelling.
	Keywords map[string]float64 `json:"keyword_inputs"`
	//nolint:tagliatelle // LIB-05: portable failure-state spelling.
	ErrorState json.RawMessage `json:"error_drive_state"`
	Entropy    uploadEntropy   `json:"entropy"`
}

type uploadEntropy struct {
	//nolint:tagliatelle // LIB-05: portable clock spelling.
	Seconds int64 `json:"unix_seconds"`
}

type uploadFileInput struct {
	Name     string `json:"name"`
	Body     string `json:"body"`
	Position int64  `json:"position"`
}

func TestDriveSDKPortableUploads(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/drive-upload-*.json")
	if err != nil || len(paths) != 9 {
		t.Fatal("upload scenario inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()

			data, readErr := os.ReadFile(filepath.Clean(path))
			if readErr != nil {
				t.Fatal(readErr)
			}

			var scenario driveUploadScenario

			decodeErr := json.Unmarshal(data, &scenario)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}

			runSDKUpload(t, scenario)
		})
	}
}

func sdkUploadRequest(t *testing.T, scenario driveUploadScenario) (icloud.UploadDriveFileRequest, *bytes.Reader) {
	t.Helper()

	var file uploadFileInput

	err := json.Unmarshal(scenario.Inputs[1], &file)
	if err != nil {
		t.Fatal(err)
	}

	content, err := base64.StdEncoding.DecodeString(file.Body)
	if err != nil {
		t.Fatal(err)
	}

	reader := bytes.NewReader(content)

	_, err = reader.Seek(file.Position, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}

	request := icloud.UploadDriveFileRequest{Auth: sdkAccountAuth(scenario.Initial), Content: reader,
		Filename: file.Name, ParentID: "", Zone: nil, CreationTime: nil, ModificationTime: nil}
	request.Auth.DriveDocumentServiceURL = scenario.Initial.DocumentOrigin

	err = json.Unmarshal(scenario.Inputs[0], &request.ParentID)
	if err != nil {
		t.Fatal(err)
	}

	if len(scenario.Inputs) > 2 {
		var zone string

		err = json.Unmarshal(scenario.Inputs[2], &zone)
		if err != nil {
			t.Fatal(err)
		}

		request.Zone = &zone
	}

	request.CreationTime = uploadInputTime(scenario.Keywords, "ctime")
	request.ModificationTime = uploadInputTime(scenario.Keywords, "mtime")

	return request, reader
}

func uploadInputTime(values map[string]float64, name string) *time.Time {
	seconds, exists := values[name]
	if !exists {
		return nil
	}

	value := time.UnixMilli(int64(seconds * 1000))

	return &value
}

func runSDKUpload(t *testing.T, scenario driveUploadScenario) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithClock(func() time.Time {
		return time.Unix(scenario.Entropy.Seconds, 0)
	}))
	if err != nil {
		t.Fatal(err)
	}

	request, reader := sdkUploadRequest(t, scenario)

	result, err := client.UploadDriveFile(t.Context(), request)
	if len(scenario.Error) != 0 {
		checkSDKUploadFailure(t, scenario, result, err)
	} else {
		if err != nil || result == nil {
			t.Fatalf("SDK upload: %v", err)
		}

		checkSDKUploadResult(t, scenario, result)
	}

	checkSDKUploadCursor(t, scenario, reader, result, err)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkSDKUploadFailure(t *testing.T, scenario driveUploadScenario,
	result *icloud.UploadDriveFileResult, err error,
) {
	t.Helper()

	var failure *icloud.ClientError

	if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.Unavailable {
		t.Fatalf("upload lost provider failure: %v", err)
	}

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	accountProviderFailure(t, scenario.Error, failure.StatusCode(), failure.ResponseBody())
	checkSDKMetadata(t, icloud.ResponseMetadata{CookieScopeURL: failure.CookieScopeURL(),
		StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders()}, last)

	prior := failure.PriorResponses()
	if len(prior) != len(scenario.Exchanges)-1 {
		t.Fatal("upload lost completed-stage authentication updates")
	}

	for index, metadata := range prior {
		checkSDKMetadata(t, metadata, scenario.Exchanges[index].Response)
	}
}

func checkSDKUploadResult(t *testing.T, scenario driveUploadScenario, result *icloud.UploadDriveFileResult) {
	t.Helper()

	checkSDKMetadata(t, result.PreparationMetadata, scenario.Exchanges[0].Response)
	checkSDKMetadata(t, result.TransferMetadata, scenario.Exchanges[1].Response)
	checkSDKMetadata(t, result.RegistrationMetadata, scenario.Exchanges[2].Response)
	receipt := sdkResponseFields(t, scenario.Exchanges[1].Response)
	checkSDKValue(t, result.UploadedFile, receipt[replayLiteralSingleFile])
	delete(receipt, replayLiteralSingleFile)

	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, result.AdditionalReceiptMetadata, encoded)

	encodedDestination, ok := accountJSON(t, scenario.Exchanges[0].Response.Body.Value).(string)
	if !ok {
		t.Fatal("destination response is not encoded bytes")
	}

	destinationBody, decodeErr := base64.StdEncoding.DecodeString(encodedDestination)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	var destinations []map[string]string

	decodeErr = json.Unmarshal(destinationBody, &destinations)
	if decodeErr != nil || len(destinations) == 0 || result.DocumentID != destinations[0][replayLiteralDocumentID] {
		t.Fatal("upload selected a different document", decodeErr)
	}

	checkSDKUploadRegistration(t, scenario, result)
}

func checkSDKUploadCursor(t *testing.T, scenario driveUploadScenario, reader *bytes.Reader,
	upload *icloud.UploadDriveFileResult, uploadErr error,
) {
	t.Helper()

	expectedState := scenario.Result
	if len(scenario.Error) != 0 {
		expectedState = scenario.ErrorState
	}

	if len(expectedState) == 0 || string(expectedState) == reminderChangeNullValue {
		return
	}

	var result struct {
		//nolint:tagliatelle // LIB-05: portable file-state spelling.
		Position int64             `json:"file_position"`
		Params   map[string]string `json:"params"`
		Value    any               `json:"value"`
	}

	err := json.Unmarshal(expectedState, &result)
	if err != nil {
		t.Fatal(err)
	}

	position, err := reader.Seek(0, io.SeekCurrent)
	if err != nil || position != result.Position {
		t.Fatal("upload changed the reference cursor outcome", err)
	}

	token := sdkUploadToken(upload, uploadErr)

	params := maps.Clone(scenario.Initial.Params)
	params["token"] = token

	encoded, encodeErr := json.Marshal(result.Params)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}

	checkSDKValue(t, params, encoded)

	if result.Value != nil {
		t.Fatal("unexpected Source upload result value")
	}
}

func checkSDKUploadRegistration(t *testing.T, scenario driveUploadScenario, result *icloud.UploadDriveFileResult) {
	t.Helper()

	fields := sdkResponseFields(t, scenario.Exchanges[2].Response)
	if records, exists := fields[replayLiteralDocuments]; exists {
		var documents []map[string]json.RawMessage

		err := json.Unmarshal(records, &documents)
		if err != nil {
			t.Fatal(err)
		}

		for _, document := range documents {
			if value, found := document[replayLiteralDocumentID]; found {
				document["documentID"] = value
				delete(document, replayLiteralDocumentID)
			}
		}

		encoded, err := json.Marshal(documents)
		if err != nil {
			t.Fatal(err)
		}

		checkSDKValue(t, result.Documents, encoded)
	} else if result.Documents != nil {
		t.Fatal("registration invented documents")
	}

	if status, exists := fields["status"]; exists {
		checkSDKValue(t, result.Status, status)
	} else if result.Status != nil {
		t.Fatal("registration invented status")
	}

	delete(fields, replayLiteralDocuments)
	delete(fields, "status")

	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, result.AdditionalRegistrationMetadata, encoded)
}

func sdkUploadToken(upload *icloud.UploadDriveFileResult, err error) string {
	if upload != nil {
		return upload.UploadToken
	}

	var failure *icloud.ClientError
	if errors.As(err, &failure) {
		return failure.UploadToken()
	}

	return ""
}
