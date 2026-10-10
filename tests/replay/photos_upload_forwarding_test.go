package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const photoUploadOpaqueInteger = "9007199254740993"

func TestPhotoUploadForwardsOpaqueReceiptWithoutRounding(t *testing.T) {
	t.Parallel()
	path := "fixtures/synthetic/http/photos-upload-service-ready.json"
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)
	scenario.Exchanges = scenario.Exchanges[:photoControlUploadStages]
	photoUploadOpaqueResponse(t, scenario.Exchanges[2].Response)
	photoUploadOpaqueRequest(t, &scenario.Exchanges[3].Request)
	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	runner := newPhotoUploadReplay(t, row, scenario, transport)
	request := runner.uploadRequest(t)
	request.Hydrate = false
	result, err := runner.client.UploadPhoto(t.Context(), request)
	if err != nil || result == nil {
		t.Fatal("high-level registration changed opaque receipt", err)
	}

	checkPhotoUploadResponses(t, result.Responses, scenario.Exchanges)
	checkPhotoUploadControlConsumed(t, transport)
}

func TestPhotoUploadComposableReceiptForwardsWithoutRounding(t *testing.T) {
	t.Parallel()
	bytesPath := "fixtures/synthetic/http/photos-upload-bytes-binary.json"
	bytesRow := authReplayObject(t, bytesPath)
	bytesScenario := readAccountScenario(t, bytesPath)
	registerPath := "fixtures/synthetic/http/photos-upload-register-1.json"
	registerRow := authReplayObject(t, registerPath)
	registerScenario := readAccountScenario(t, registerPath)
	photoUploadOpaqueResponse(t, bytesScenario.Exchanges[1].Response)
	photoUploadOpaqueRequest(t, &registerScenario.Exchanges[1].Request)
	exchanges := append(bytesScenario.Exchanges, registerScenario.Exchanges...)
	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	runner := newPhotoUploadReplay(t, bytesRow, bytesScenario, transport)
	var inputs []string

	authReplayDecode(t, bytesRow["inputs"], &inputs)
	_, content, _ := photoUploadContent(t, bytesRow)
	sent, err := runner.client.SendPhotoUploadBytes(t.Context(), icloud.SendPhotoUploadBytesRequest{
		Auth: runner.auth, Library: nil, URL: inputs[0], Content: content})
	if err != nil || sent == nil || string(sent.Receipt.AdditionalProperties["opaque"]) != photoUploadOpaqueInteger {
		t.Fatal("composable transfer changed opaque receipt", err)
	}

	request := photoUploadReceiptRegistration(t, registerRow, runner.auth, sent.Receipt)
	before := marshalFindMyRecovery(t, request)
	registered, err := runner.client.RegisterPhotoUploads(t.Context(), request)
	if err != nil || registered == nil {
		t.Fatal("composable registration changed opaque receipt", err)
	}

	checkSDKValue(t, request, before)
	checkPhotoUploadResponses(t, sent.Responses, bytesScenario.Exchanges)
	checkPhotoUploadResponses(t, registered.Responses, registerScenario.Exchanges)
	checkPhotoUploadControlConsumed(t, transport)
}

func photoUploadOpaqueResponse(t *testing.T, response *replay.Response) {
	t.Helper()
	body := photoUploadOpaqueBody(t, response.Body)
	response.Body.Value = marshalFindMyRecovery(t, base64.StdEncoding.EncodeToString(body))
}

func photoUploadOpaqueRequest(t *testing.T, request *replay.Request) {
	t.Helper()
	body := photoUploadOpaqueBody(t, request.Body)
	request.Body.Value = marshalFindMyRecovery(t, base64.StdEncoding.EncodeToString(body))
	for index := range request.Headers {
		if request.Headers[index][0] == "content-length" {
			request.Headers[index][1] = strconv.Itoa(len(body))
		}
	}
}

func photoUploadOpaqueBody(t *testing.T, entity replay.Entity) []byte {
	t.Helper()
	const marker = `"receipt": "synthetic-receipt"`
	body := contractAuthBody(t, entity)
	if bytes.Count(body, []byte(marker)) != 1 {
		t.Fatal("opaque forwarding control requires exactly one receipt")
	}

	return bytes.Replace(body, []byte(marker), []byte(marker+`, "opaque": `+photoUploadOpaqueInteger), 1)
}

func photoUploadReceiptRegistration(t *testing.T, row map[string]json.RawMessage,
	auth icloud.AuthContext, receipt icloud.PhotoUploadReceipt,
) icloud.RegisterPhotoUploadsRequest {
	t.Helper()
	var keywords map[string]json.RawMessage
	var files []map[string]json.RawMessage

	authReplayDecode(t, row["keyword_inputs"], &keywords)
	authReplayDecode(t, keywords["files"], &files)
	file := new(icloud.PhotoUploadFile)
	file.Receipt = receipt
	authReplayDecode(t, files[0]["fileName"], &file.Filename)
	authReplayDecode(t, files[0]["timeZoneOffset"], &file.TimeZoneOffset)
	var milliseconds int64

	authReplayDecode(t, files[0]["lastModDate"], &milliseconds)
	file.ModificationTime = time.UnixMilli(milliseconds)
	request := icloud.RegisterPhotoUploadsRequest{Auth: auth, Library: nil, Files: []icloud.PhotoUploadFile{*file},
		ImportGroup: "", LocalTimeZoneID: ""}
	authReplayDecode(t, keywords["import_group"], &request.ImportGroup)
	authReplayDecode(t, keywords["local_time_zone_id"], &request.LocalTimeZoneID)

	return request
}
