package command_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

type readProbe struct {
	icloud.Client
	calls  int
	auth   icloud.AuthContext
	result *icloud.GetPhotoUploadStatusResult
}

func (probe *readProbe) GetPhotoUploadStatus(_ context.Context, request icloud.GetPhotoUploadStatusRequest) (*icloud.GetPhotoUploadStatusResult, error) {
	probe.calls++
	probe.auth = request.Auth
	return probe.result, nil
}

func TestTypedReadUnknownProviderFieldsRemainPrivate(t *testing.T) {
	t.Parallel()
	var status icloud.PhotoUploadStatus
	status.Progress = nullable.NewNullableWithValue(100)
	status.ErrorCode.SetNull()
	status.AdditionalProperties = map[string]icloud.UnknownJSONValue{"unreviewedProviderField": json.RawMessage(`"synthetic-private-status"`)}
	var result icloud.GetPhotoUploadStatusResult
	result.Jobs = map[string]icloud.PhotoUploadStatus{"synthetic-job": status}
	result.Responses = []icloud.ResponseMetadata{}
	probe := &readProbe{Client: nil, calls: 0, auth: resultAuth(), result: &result}
	directory := t.TempDir()
	session, request, saved := filepath.Join(directory, "session.json"), filepath.Join(directory, "request.json"), filepath.Join(directory, "result.json")
	auth := resultAuth()
	writeFixtureValue(t, session, auth)
	writeFixtureValue(t, request, map[string]any{"auth": map[string]any{"accountID": "foreign"}, "jobIDs": []string{"synthetic-job"}})
	original, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var output, diagnostic bytes.Buffer
	err = command.Run(t.Context(), probe, []string{"--session", session, "--request", request, "--save-result", saved, "photo-upload-status"}, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if probe.calls != 1 || !reflect.DeepEqual(probe.auth, auth) {
		t.Fatal("stored authentication was not selected")
	}
	if strings.Contains(output.String(), "synthetic-private-status") || strings.Contains(output.String(), "unreviewedProviderField") {
		t.Fatal("opaque provider fields reached console")
	}
	private, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(private, original) {
		t.Fatal("explicit private result differs from original")
	}
	after, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatal("console projection mutated original SDK result")
	}
}

func resultAuth() icloud.AuthContext {
	var auth icloud.AuthContext
	auth.AccountID = "stored-account"
	auth.ClientID = "stored-client"
	auth.PhotosServiceURL = "https://photos.example.invalid"
	auth.Headers = []icloud.Header{{Name: "Authorization", Value: writeSecret}}
	return auth
}
