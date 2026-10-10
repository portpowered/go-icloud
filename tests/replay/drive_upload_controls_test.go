package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type uploadShapeCase struct {
	Stage int
	Body  string
}

func TestDriveUploadStopsBeforeNextWriteOnIncompleteReply(t *testing.T) {
	t.Parallel()

	cases := map[string]uploadShapeCase{
		"missing-document": {Stage: 0, Body: `[{"url":"https://content.example.invalid/upload"}]`},
		"null-document":    {Stage: 0, Body: `[{"document_id":null,"url":"https://content.example.invalid/upload"}]`},
		"missing-url":      {Stage: 0, Body: `[{"document_id":"synthetic"}]`},
		"wrong-url-type":   {Stage: 0, Body: `[{"document_id":"synthetic","url":true}]`},
		"empty-array":      {Stage: 0, Body: `[]`},
		"missing-file":     {Stage: 1, Body: `{}`},
		"null-file":        {Stage: 1, Body: `{"singleFile":null}`},
		"empty-file":       {Stage: 1, Body: `{"singleFile":{}}`},
		"missing-checksum": {Stage: 1, Body: `{"singleFile":{"wrappingKey":"k","referenceChecksum":"r","size":0}}`},
		"missing-key":      {Stage: 1, Body: `{"singleFile":{"fileChecksum":"c","referenceChecksum":"r","size":0}}`},
		"missing-reference": {Stage: 1,
			Body: `{"singleFile":{"fileChecksum":"c","wrappingKey":"k","size":0}}`},
		"missing-size": {Stage: 1,
			Body: `{"singleFile":{"fileChecksum":"c","wrappingKey":"k","referenceChecksum":"r"}}`},
		"null-size": {Stage: 1,
			Body: `{"singleFile":{"fileChecksum":"c","wrappingKey":"k","referenceChecksum":"r","size":null}}`},
		"wrong-size-type": {Stage: 1,
			Body: `{"singleFile":{"fileChecksum":"c","wrappingKey":"k","referenceChecksum":"r","size":"0"}}`},
	}

	for name, sample := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkSDKUploadShapeFailure(t, sample)
		})
	}
}

func checkSDKUploadShapeFailure(t *testing.T, sample uploadShapeCase) {
	t.Helper()

	scenario, transport := uploadShapeTransport(t, sample)

	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithClock(func() time.Time {
		return time.Unix(scenario.Entropy.Seconds, 0)
	}))
	if err != nil {
		t.Fatal(err)
	}

	request, _ := sdkUploadRequest(t, scenario)
	result, err := client.UploadDriveFile(t.Context(), request)

	var failure *icloud.ClientError

	if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse ||
		failure.StatusCode() != 200 || string(failure.ResponseBody()) != sample.Body ||
		len(failure.PriorResponses()) != sample.Stage || failure.UploadToken() != "synthetic-upload" {
		t.Fatalf("incomplete provider reply reached another write or lost response evidence: %v", err)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func uploadShapeTransport(t *testing.T, sample uploadShapeCase) (driveUploadScenario, *replay.HTTPTransport) {
	t.Helper()
	scenario := driveUploadScenario{driveReadScenario: driveReadScenario{
		accountScenario: readAccountScenario(t, "fixtures/synthetic/http/drive-upload-observed.json"), Inputs: nil,
	}, Keywords: nil, Entropy: uploadEntropy{Seconds: 1700000000}, ErrorState: nil}
	// Keep the control's caller input identical to the portable positive case.
	input, err := json.Marshal(uploadFileInput{Name: replayLiteralSyntheticTxt,
		Body: "AHN5bnRoZXRpYyB1cGxvYWT/", Position: 0})
	if err != nil {
		t.Fatal(err)
	}

	scenario.Inputs = []json.RawMessage{json.RawMessage(`"FOLDER::synthetic::root"`), input}
	scenario.Exchanges = scenario.Exchanges[:sample.Stage+1]

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString([]byte(sample.Body)))
	if err != nil {
		t.Fatal(err)
	}

	scenario.Exchanges[sample.Stage].Response.Body.Value = encoded

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	return scenario, transport
}
