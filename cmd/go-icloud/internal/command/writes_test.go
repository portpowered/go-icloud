package command_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const writeSecret = "synthetic-private-secret"

type writeProbe struct {
	icloud.Client
	calls    int
	received icloud.AuthContext
	cancel   context.CancelFunc
	content  io.ReadSeeker
}

func (probe *writeProbe) CreateReminder(_ context.Context, input icloud.CreateReminderRequest) (*icloud.ReminderMutationResult, error) {
	probe.calls++
	probe.received = input.Auth
	if probe.cancel != nil {
		probe.cancel()
	}

	return &icloud.ReminderMutationResult{Reminder: icloud.Reminder{Title: input.Title},
		Responses: []icloud.ResponseMetadata{{Headers: []icloud.Header{{Name: "Set-Cookie", Value: writeSecret}}}}}, nil
}

func (probe *writeProbe) ReservePhotoUploads(_ context.Context, input icloud.ReservePhotoUploadsRequest) (*icloud.ReservePhotoUploadsResult, error) {
	probe.calls++
	probe.received = input.Auth

	return &icloud.ReservePhotoUploadsResult{UploadURLs: map[string]string{"file": "https://content.example.invalid/?token=" + writeSecret}}, nil
}

func (probe *writeProbe) UploadPhotoFile(_ context.Context, input icloud.UploadPhotoFileRequest) (*icloud.UploadPhotoFileResult, error) {
	probe.calls++
	probe.received = input.Auth
	probe.content = input.Content
	_, err := io.Copy(io.Discard, input.Content)
	if err != nil {
		return nil, err
	}

	return &icloud.UploadPhotoFileResult{}, nil
}

func TestTypedWriteInputFailures(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"malformed":    "{",
		"null":         "null",
		"unknown":      `{"title":"safe","privateUnknown":"synthetic-private-secret"}`,
		"unknown-auth": `{"title":"safe","auth":{"privateUnknown":"synthetic-private-secret"}}`,
		"multiple":     `{"title":"first"} {"title":"second"}`,
		"duplicate":    `{"title":"first","title":"second"}`,
		"invalid-utf8": "{\"title\":\"" + string([]byte{0xff}) + "\"}",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			probe := new(writeProbe)
			_, output, err := runWriteProbe(t, t.Context(), probe, "reminder-create", body, nil)
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
	auth, output, err := runWriteProbe(t, t.Context(), probe, "reminder-create",
		`{"title":"safe","auth":{"clientID":"foreign","accountID":"foreign","headers":[]}}`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(probe.received, auth) || probe.calls != 1 {
		t.Fatal("request replaced stored account boundary")
	}
	if strings.Contains(output, writeSecret) || strings.Contains(output, "responses") {
		t.Fatal("credential response leaked into console")
	}
}

func TestUploadPhaseRequiresExplicitPrivateResult(t *testing.T) {
	t.Parallel()
	probe := new(writeProbe)
	_, output, err := runWriteProbe(t, t.Context(), probe, "photo-upload-reserve", `{"assets":{"file":3}}`, nil)
	if err == nil || !strings.Contains(err.Error(), "--save-result") || probe.calls != 0 || output != "" {
		t.Fatalf("phase executed without private receipt destination: %v", err)
	}
}

func TestPrivateUploadResultRetainsSignedDestination(t *testing.T) {
	t.Parallel()
	probe := new(writeProbe)
	destination := filepath.Join(t.TempDir(), "receipt.json")
	_, output, err := runWriteProbe(t, t.Context(), probe, "photo-upload-reserve",
		`{"assets":{"file":3}}`, []string{"--save-result", destination})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(saved, []byte(writeSecret)) || strings.Contains(output, writeSecret) || strings.Contains(output, "uploadURLs") {
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
	probe := &writeProbe{Client: nil, calls: 0, received: icloud.AuthContext{}, cancel: cancel, content: nil}
	_, output, err := runWriteProbe(t, ctx, probe, "reminder-create", `{"title":"safe"}`,
		[]string{"--save-result", destination})
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
	content := filepath.Join(t.TempDir(), "content.jpg")
	writeProbeFile(t, content, []byte("synthetic photo"))
	probe := new(writeProbe)
	_, _, err := runWriteProbe(t, t.Context(), probe, "photo-upload-file",
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

func TestUnknownAttachmentUnionFieldRejected(t *testing.T) {
	t.Parallel()
	probe := new(writeProbe)
	_, output, err := runWriteProbe(t, t.Context(), probe, "reminder-attachment-update",
		`{"attachment":{"url":"https://example.invalid","privateUnknown":"synthetic-private-secret"}}`, nil)
	var failure *command.WriteRequestError
	if !errors.As(err, &failure) || output != "" {
		t.Fatalf("generated union accepted unknown fields: %v", err)
	}
}

func runWriteProbe(t *testing.T, ctx context.Context, client icloud.Client, operation, request string,
	flags []string,
) (icloud.AuthContext, string, error) {
	t.Helper()
	directory := t.TempDir()
	auth := icloud.AuthContext{AccountID: "stored-account", ClientID: "stored-client",
		PhotosServiceURL: "https://photos.example.invalid", RemindersServiceURL: "https://reminders.example.invalid",
		Headers: []icloud.Header{{Name: "Authorization", Value: writeSecret}}}
	data, err := json.Marshal(auth)
	if err != nil {
		t.Fatal(err)
	}
	session := filepath.Join(directory, "session.json")
	input := filepath.Join(directory, "request.json")
	writeProbeFile(t, session, data)
	writeProbeFile(t, input, []byte(request))
	args := append([]string{"--session", session, "--request", input}, flags...)
	args = append(args, operation)
	var output, diagnostic bytes.Buffer
	err = command.Run(ctx, client, args, &output, &diagnostic)

	return auth, output.String(), err
}

func writeProbeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
