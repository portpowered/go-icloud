package command_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/oapi-codegen/nullable"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const writeSecret = "synthetic-private-secret"

type writeProbe struct {
	icloud.Client

	calls        int
	received     icloud.AuthContext
	cancel       context.CancelFunc
	content      io.ReadSeeker
	uploadResult *icloud.UploadPhotoFileResult
}

func (probe *writeProbe) CreateReminder(
	_ context.Context, input icloud.CreateReminderRequest,
) (*icloud.ReminderMutationResult, error) {
	probe.calls++
	probe.received = input.Auth
	if probe.cancel != nil {
		probe.cancel()
	}

	var reminder icloud.Reminder

	reminder.Title = input.Title

	return &icloud.ReminderMutationResult{Reminder: reminder,
		Responses: []icloud.ResponseMetadata{{StatusCode: 0, CookieScopeURL: "",
			Headers: []icloud.Header{{Name: "Set-Cookie", Value: writeSecret}}}}}, nil
}

func (probe *writeProbe) ReservePhotoUploads(
	_ context.Context, input icloud.ReservePhotoUploadsRequest,
) (*icloud.ReservePhotoUploadsResult, error) {
	probe.calls++
	probe.received = input.Auth

	return &icloud.ReservePhotoUploadsResult{
		UploadURLs: map[string]string{"file": "https://content.example.invalid/?token=" + writeSecret}, Responses: nil,
	}, nil
}

func (probe *writeProbe) UploadPhotoFile(
	_ context.Context, input icloud.UploadPhotoFileRequest,
) (*icloud.UploadPhotoFileResult, error) {
	probe.calls++
	probe.received = input.Auth
	probe.content = input.Content
	_, err := io.Copy(io.Discard, input.Content)
	if err != nil {
		return nil, fmt.Errorf("consume synthetic upload content: %w", err)
	}
	if probe.uploadResult != nil {
		return probe.uploadResult, nil
	}

	return new(icloud.UploadPhotoFileResult), nil
}

func TestTypedWriteInputFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"malformed":      "{",
		"null":           "null",
		"unknown":        `{"title":"safe","privateUnknown":"synthetic-private-secret"}`,
		"unknown-auth":   `{"title":"safe","auth":{"privateUnknown":"synthetic-private-secret"}}`,
		"multiple":       `{"title":"first"} {"title":"second"}`,
		testDuplicateKey: `{"title":"first","title":"second"}`,
		"invalid-utf8":   "{\"title\":\"" + string([]byte{0xff}) + "\"}",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			probe := new(writeProbe)
			_, output, err := runWriteProbe(t.Context(), t, probe, testReminderCreateCommand, body, nil)

			var failure *command.WriteRequestError

			if !errors.As(err, &failure) || probe.calls != 0 || output != "" {
				t.Fatalf("invalid typed input accepted: %v", err)
			}
			if strings.Contains(err.Error(), writeSecret) {
				t.Fatal("private input leaked into diagnostic")
			}
		})
	}
}

func TestWriteUsesCompleteStoredAuthentication(t *testing.T) {
	t.Parallel()
	probe := new(writeProbe)
	auth, output, err := runWriteProbe(t.Context(), t, probe, testReminderCreateCommand,
		`{"title":"safe","auth":{"clientID":"foreign","accountID":"foreign","headers":[]}}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(probe.received, auth) || probe.calls != 1 {
		t.Fatal("request replaced stored account boundary")
	}
	if strings.Contains(output, writeSecret) || strings.Contains(output, testResponsesKey) {
		t.Fatal("credential response leaked into console")
	}
}

func TestUploadPhaseRequiresExplicitPrivateResult(t *testing.T) {
	t.Parallel()
	probe := new(writeProbe)
	_, output, err := runWriteProbe(t.Context(), t, probe, testPhotoUploadReserveCommand, `{"assets":{"file":3}}`, nil)
	if err == nil || !strings.Contains(err.Error(), testSaveResultFlag) || probe.calls != 0 || output != "" {
		t.Fatalf("phase executed without private receipt destination: %v", err)
	}
}

func TestPrivateUploadResultRetainsSignedDestination(t *testing.T) {
	t.Parallel()
	probe := new(writeProbe)
	destination := filepath.Join(t.TempDir(), "receipt.json")
	_, output, err := runWriteProbe(t.Context(), t, probe, testPhotoUploadReserveCommand,
		`{"assets":{"file":3}}`, []string{testSaveResultFlag, destination})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(saved, []byte(writeSecret)) ||
		strings.Contains(output, writeSecret) || strings.Contains(output, testUploadURLsKey) {
		t.Fatal("private upload receipt and console projection were conflated")
	}
}

func TestCanceledWritePreservesPrivateResult(t *testing.T) {
	t.Parallel()
	destination := filepath.Join(t.TempDir(), "existing.json")
	original := []byte("existing private result")
	writeProbeFile(t, destination, original)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	probe := new(writeProbe)
	probe.cancel = cancel
	_, output, err := runWriteProbe(ctx, t, probe, testReminderCreateCommand, `{"title":"safe"}`,
		[]string{testSaveResultFlag, destination})
	if !errors.Is(err, context.Canceled) || output != "" {
		t.Fatalf("canceled result was published: %v", err)
	}
	saved, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(saved, original) {
		t.Fatal("cancellation replaced existing private result")
	}
}

func TestUploadOwnsLocalFile(t *testing.T) {
	t.Parallel()
	content := filepath.Join(t.TempDir(), testContentJpgFilename)
	writeProbeFile(t, content, []byte(testSyntheticPhotoContent))
	probe := new(writeProbe)
	_, _, err := runWriteProbe(t.Context(), t, probe, testPhotoUploadFileCommand,
		`{"filename":"content.jpg","localTimeZoneID":"UTC","modificationTime":"2023-11-14T22:13:20Z","timeZoneOffset":0}`,
		[]string{"--file", content})
	if err != nil {
		t.Fatal(err)
	}
	_, err = probe.content.Read(make([]byte, 1))
	if !errors.Is(err, fs.ErrClosed) {
		t.Fatal("CLI left its local file open")
	}
}

func TestWriteUnknownProviderFieldsOnlyInPrivateResult(t *testing.T) {
	t.Parallel()

	var status icloud.PhotoUploadRegistrationStatus

	status.Status.Set(200)
	status.ErrorMessage.Set(writeSecret)
	status.AdditionalProperties = map[string]icloud.UnknownJSONValue{
		testOpaqueStatusKey: json.RawMessage(`"synthetic-private-secret"`),
	}

	var registration icloud.PhotoUploadRegistration

	registration.Status = nullable.NewNullableWithValue(status)
	registration.AdditionalProperties = map[string]icloud.UnknownJSONValue{
		testOpaqueReceiptKey: json.RawMessage(`"synthetic-private-secret"`),
	}

	var result icloud.UploadPhotoFileResult

	result.Registration = registration
	probe := new(writeProbe)
	probe.uploadResult = &result
	before, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	content := filepath.Join(t.TempDir(), testContentJpgFilename)
	destination := filepath.Join(t.TempDir(), "private-result.json")
	writeProbeFile(t, content, []byte(testSyntheticPhotoContent))
	_, output, err := runWriteProbe(t.Context(), t, probe, testPhotoUploadFileCommand,
		`{"filename":"content.jpg","localTimeZoneID":"UTC","modificationTime":"2023-11-14T22:13:20Z","timeZoneOffset":0}`,
		[]string{"--file", content, testSaveResultFlag, destination})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !bytes.Equal(before, saved) {
		t.Fatal("projection changed original generated SDK result")
	}
	if strings.Contains(output, writeSecret) ||
		strings.Contains(output, testOpaqueStatusKey) || strings.Contains(output, testOpaqueReceiptKey) {
		t.Fatal("uninterpreted provider fields leaked into console")
	}
}

func TestUnknownAttachmentUnionFieldRejected(t *testing.T) {
	t.Parallel()
	probe := new(writeProbe)
	_, output, err := runWriteProbe(t.Context(), t, probe, testReminderAttachmentUpdateCommand,
		`{"attachment":{"url":"https://example.invalid","privateUnknown":"synthetic-private-secret"}}`, nil)

	var failure *command.WriteRequestError

	if !errors.As(err, &failure) || output != "" {
		t.Fatalf("generated union accepted unknown fields: %v", err)
	}
}

func runWriteProbe(ctx context.Context, t *testing.T, client icloud.Client, operation, request string,
	flags []string,
) (icloud.AuthContext, string, error) {
	t.Helper()
	directory := t.TempDir()

	var auth icloud.AuthContext

	auth.AccountID, auth.ClientID = testStoredAccount, testStoredClient
	auth.PhotosServiceURL = testPhotosServiceURL
	auth.RemindersServiceURL = "https://reminders.example.invalid"
	auth.Headers = []icloud.Header{{Name: testAuthorizationKey, Value: writeSecret}}
	//nolint:gosec // GO-15: synthetic authentication is deliberately persisted to exercise the private session boundary.
	data, err := json.Marshal(auth)
	if err != nil {
		t.Fatal(err)
	}
	session := filepath.Join(directory, testSessionJsonFilename)
	input := filepath.Join(directory, testRequestJsonFilename)

	writeProbeFile(t, session, data)
	writeProbeFile(t, input, []byte(request))
	args := append([]string{testSessionFlag, session, testRequestFlag, input}, flags...)
	args = append(args, operation)

	var output, diagnostic bytes.Buffer

	err = command.Run(ctx, client, args, &output, &diagnostic)

	return auth, output.String(), err
}

func writeProbeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	err := os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
