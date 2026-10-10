package command_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const (
	firstVisitorPhoto  = "first"
	secondVisitorPhoto = "second"
)

var (
	errVisitorForeignAuth = errors.New("foreign authentication reached SDK")
	errVisitorStopped     = errors.New("visitor unexpectedly stopped")
	errVisitorAsync       = errors.New("visitor did not emit synchronously")
	errPhotoWriter        = errors.New("synthetic writer failure")
)

type visitorProbe struct {
	icloud.Client

	want   icloud.AuthContext
	output *bytes.Buffer
	cancel context.CancelFunc
	visits int
}

type failingPhotoWriter struct{ failure error }

func (output failingPhotoWriter) Write(_ []byte) (int, error) { return 0, output.failure }

func (probe *visitorProbe) VisitPhotoAssets(_ context.Context, request icloud.ListPhotoAssetsRequest,
	visitor icloud.PhotoVisitor,
) (*icloud.ListPhotoAssetsResult, error) {
	err := probe.deliver(request.Auth, visitor)
	if err != nil {
		return nil, err
	}

	var result icloud.ListPhotoAssetsResult

	result.Photos = []icloud.Photo{}
	result.Responses = []icloud.ResponseMetadata{}
	return &result, nil
}

func (probe *visitorProbe) VisitRecentlyAddedPhotos(_ context.Context, request icloud.ListRecentlyAddedPhotosRequest,
	visitor icloud.PhotoVisitor,
) (*icloud.ListRecentlyAddedPhotosResult, error) {
	err := probe.deliver(request.Auth, visitor)
	if err != nil {
		return nil, err
	}

	var result icloud.ListRecentlyAddedPhotosResult

	result.Photos = []icloud.Photo{}
	result.Responses = []icloud.ResponseMetadata{}
	return &result, nil
}

func (probe *visitorProbe) deliver(auth icloud.AuthContext, visitor icloud.PhotoVisitor) error {
	if !reflect.DeepEqual(auth, probe.want) {
		return errVisitorForeignAuth
	}
	if probe.cancel != nil {
		probe.cancel()
	}

	for _, identifier := range []string{firstVisitorPhoto, secondVisitorPhoto} {
		var event icloud.PhotoVisitEvent

		event.Photo.ID = identifier
		event.Photo.AssetMetadata = json.RawMessage(`{"private":"synthetic-stream-secret"}`)
		keepGoing, err := visitor(event)
		if err != nil {
			return err
		}
		if !keepGoing {
			return errVisitorStopped
		}
		probe.visits++
		if probe.output != nil && !strings.Contains(probe.output.String(), identifier) {
			return errVisitorAsync
		}
	}
	return nil
}

func TestPhotoVisitorsStreamBeforeNextCallback(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{testPhotoAssetsVisitCommand, "photos-recently-added-visit"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer

			var probe visitorProbe

			probe.want = resultAuth()
			probe.want.AccountID = "stored"
			probe.output = &output
			session, input, saved := visitorPaths(t, probe.want)

			err := runVisitorCLI(t.Context(), &probe, session, operation, input, saved, &output)
			if err != nil {
				t.Fatal(err)
			}
			if probe.visits != 2 || strings.Contains(output.String(), "synthetic-stream-secret") {
				t.Fatal("stream count or privacy differs")
			}

			checkVisitorStream(t, &output)
		})
	}
}

func TestPhotoVisitorCanceledBeforeEmissionPreservesReceipt(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var probe visitorProbe

	probe.want = resultAuth()
	probe.cancel = cancel
	session, input, saved := visitorPaths(t, probe.want)
	previous := []byte(testPreviousReceipt)

	writeErr := os.WriteFile(saved, previous, 0o600)
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	var output bytes.Buffer

	err := runVisitorCLI(ctx, &probe, session, testPhotoAssetsVisitCommand, input, saved, &output)
	if !errors.Is(err, context.Canceled) || output.Len() != 0 || probe.visits != 0 {
		t.Fatal("cancellation did not stop emission", err)
	}
	actual := readVisitorReceipt(t, saved)
	if !bytes.Equal(actual, previous) {
		t.Fatal("cancellation overwrote existing receipt")
	}
}

func TestPhotoVisitorStopsOnOutputFailure(t *testing.T) {
	t.Parallel()

	var probe visitorProbe

	probe.want = resultAuth()
	session, input, saved := visitorPaths(t, probe.want)
	previous := []byte(testPreviousReceipt)

	writeErr := os.WriteFile(saved, previous, 0o600)
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	failure := errPhotoWriter
	err := runVisitorCLI(t.Context(), &probe, session, testPhotoAssetsVisitCommand, input, saved,
		failingPhotoWriter{failure: failure})
	if !errors.Is(err, failure) || probe.visits != 0 {
		t.Fatal("output failure did not stop SDK visitor", err)
	}
	actual := readVisitorReceipt(t, saved)
	if !bytes.Equal(actual, previous) {
		t.Fatal("failed stream overwrote receipt")
	}
}

func visitorPaths(t *testing.T, auth icloud.AuthContext) (string, string, string) {
	t.Helper()
	directory := t.TempDir()
	session := filepath.Join(directory, testSessionJSONFilename)
	writeFixtureValue(t, session, auth)
	input, saved := filepath.Join(directory, testRequestJSONFilename), filepath.Join(directory, testResultJSONFilename)

	err := os.WriteFile(input, []byte(`{"auth":{"accountID":"foreign"}}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return session, input, saved
}

func runVisitorCLI(ctx context.Context, client icloud.Client, session, operation, input, saved string,
	output io.Writer,
) error {
	args := []string{testSessionFlag, session, testRequestFlag, input, testSaveResultFlag, saved, operation}
	err := command.Run(ctx, client, args, output, io.Discard)
	if err != nil {
		return fmt.Errorf("run visitor CLI: %w", err)
	}

	return nil
}

func readVisitorReceipt(t *testing.T, path string) []byte {
	t.Helper()
	directory, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = directory.Close() }()
	data, err := directory.ReadFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func checkVisitorStream(t *testing.T, output io.Reader) {
	t.Helper()
	decoder := json.NewDecoder(output)

	for _, identifier := range []string{firstVisitorPhoto, secondVisitorPhoto} {
		var event map[string]json.RawMessage

		err := decoder.Decode(&event)
		if err != nil {
			t.Fatal(err)
		}

		var photo map[string]json.RawMessage

		err = json.Unmarshal(event["photo"], &photo)
		if err != nil {
			t.Fatal(err)
		}

		var actual string

		err = json.Unmarshal(photo["id"], &actual)
		if err != nil || actual != identifier {
			t.Fatal("stream order differs", err)
		}
	}

	var summary map[string]json.RawMessage

	err := decoder.Decode(&summary)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(summary["photos"], []byte("[]")) {
		t.Fatal("stream summary differs")
	}

	var extra any

	err = decoder.Decode(&extra)
	if !errors.Is(err, io.EOF) {
		t.Fatal("unexpected stream messages")
	}
}
