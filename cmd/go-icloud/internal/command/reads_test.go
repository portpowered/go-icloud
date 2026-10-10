package command_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oapi-codegen/nullable"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestTypedReadInputFailures(t *testing.T) {
	t.Parallel()

	for _, request := range []string{`{`, `null`, `{"unreviewed":true}`, `{} {}`, `{"auth":{"unknown":"private"}}`} {
		t.Run(request, func(t *testing.T) {
			t.Parallel()

			var probe readProbe

			_, output, err := runWriteProbe(t.Context(), t, &probe, testPhotoUploadStatusCommand, request, nil)

			var inputError *command.WriteRequestError

			if !errors.As(err, &inputError) || probe.calls != 0 || output != "" {
				t.Fatal("invalid typed read reached SDK or output", err)
			}
		})
	}
}

type readProbe struct {
	icloud.Client

	calls  int
	auth   icloud.AuthContext
	result *icloud.GetPhotoUploadStatusResult
}

func (probe *readProbe) GetPhotoUploadStatus(
	_ context.Context, request icloud.GetPhotoUploadStatusRequest,
) (*icloud.GetPhotoUploadStatusResult, error) {
	probe.calls++
	probe.auth = request.Auth
	return probe.result, nil
}

func TestTypedReadUnknownProviderFieldsRemainPrivate(t *testing.T) {
	t.Parallel()

	var status icloud.PhotoUploadStatus

	status.Progress = nullable.NewNullableWithValue(100)
	status.ErrorCode.SetNull()
	status.AdditionalProperties = map[string]icloud.UnknownJSONValue{
		testUnreviewedProviderFieldKey: json.RawMessage(`"synthetic-private-status"`),
	}

	var result icloud.GetPhotoUploadStatusResult

	result.Jobs = map[string]icloud.PhotoUploadStatus{testSyntheticJob: status}
	result.Responses = []icloud.ResponseMetadata{}
	probe := &readProbe{Client: nil, calls: 0, auth: resultAuth(), result: &result}
	directory := t.TempDir()
	session := filepath.Join(directory, testSessionJsonFilename)
	request, saved := filepath.Join(directory, testRequestJsonFilename), filepath.Join(directory, testResultJsonFilename)
	auth := resultAuth()
	writeFixtureValue(t, session, auth)
	writeFixtureValue(t, request, map[string]any{
		"auth": map[string]any{testAccountIDKey: "foreign"}, "jobIDs": []string{testSyntheticJob},
	})
	original, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	var output, diagnostic bytes.Buffer

	args := []string{testSessionFlag, session, testRequestFlag, request, testSaveResultFlag, saved,
		testPhotoUploadStatusCommand}
	err = command.Run(t.Context(), probe, args, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if probe.calls != 1 || !reflect.DeepEqual(probe.auth, auth) {
		t.Fatal("stored authentication was not selected")
	}
	if strings.Contains(output.String(), "synthetic-private-status") ||
		strings.Contains(output.String(), testUnreviewedProviderFieldKey) {
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

	auth.AccountID = testStoredAccount
	auth.ClientID = testStoredClient
	auth.PhotosServiceURL = testPhotosServiceURL
	auth.Headers = []icloud.Header{{Name: testAuthorizationKey, Value: writeSecret}}
	return auth
}
