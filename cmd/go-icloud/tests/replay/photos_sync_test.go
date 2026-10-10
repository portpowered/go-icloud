package replay_test

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

	"github.com/oapi-codegen/nullable"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/photosynccommand"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
)

var errSyncOutputRejected = errors.New("synthetic output rejection")

type syncCLIClient struct {
	icloud.Client

	t         *testing.T
	cursors   int
	visits    int
	downloads int
	cancel    context.CancelFunc
	named     bool
}

func (client *syncCLIClient) GetPhotosCursor(
	_ context.Context, request icloud.GetPhotosCursorRequest,
) (*icloud.GetPhotosCursorResult, error) {
	client.t.Helper()
	if request.Auth.AccountID != "stored-account" {
		client.t.Fatal("request authentication displaced the saved native account")
	}
	if client.named && (request.Library == nil || request.Library.ID != "selected") {
		client.t.Fatal("cursor lost the discovered library selection")
	}
	client.cursors++
	return &icloud.GetPhotosCursorResult{
		SyncToken: fmt.Sprintf("cursor-%d", client.cursors), Responses: syncReceipt("cursor"),
	}, nil
}

func (client *syncCLIClient) ListPhotoLibraries(
	_ context.Context, request icloud.ListPhotoLibrariesRequest,
) (*icloud.ListPhotoLibrariesResult, error) {
	client.t.Helper()
	if !client.named || request.Auth.AccountID != "stored-account" {
		client.t.Fatal("unexpected discovery or incorrect saved authentication")
	}
	var library icloud.PhotoLibrary
	decode(client.t, json.RawMessage(`{"id":"selected","zoneName":"selected-zone","zoneType":"REGULAR_CUSTOM_ZONE","ownerRecordName":null,"shared":false,"isSharedLibrary":false,"indexingState":"FINISHED","syncToken":null}`), &library)
	return &icloud.ListPhotoLibrariesResult{
		Libraries: []icloud.PhotoLibrary{library}, Responses: syncReceipt("discovery"),
	}, nil
}

func (client *syncCLIClient) VisitPhotoAssets(ctx context.Context, _ icloud.ListPhotoAssetsRequest,
	visitor icloud.PhotoVisitor,
) (*icloud.ListPhotoAssetsResult, error) {
	client.t.Helper()
	client.visits++
	var photo icloud.Photo
	decode(client.t, json.RawMessage(`{"id":"asset","filename":"photo.jpg","masterID":"master","created":"2020-01-01T00:00:00Z","added":"2020-01-01T00:00:00Z","itemType":"image","isLivePhoto":false,"assetMetadata":{},"dimensions":[null,null],"size":1,"versions":{"original":{"url":"https://photos.example.invalid/photo","size":1,"type":"public.jpeg","checksum":"one"}}}`), &photo)
	responses := syncReceipt("page")
	_, err := visitor(icloud.PhotoVisitEvent{Photo: photo, Responses: responses})
	if err != nil {
		return nil, err
	}
	if client.cancel != nil {
		client.cancel()
		return nil, ctx.Err()
	}
	return &icloud.ListPhotoAssetsResult{Photos: []icloud.Photo{photo}, Responses: responses}, nil
}

func (client *syncCLIClient) DownloadPhoto(
	_ context.Context, request icloud.DownloadPhotoRequest,
) (*icloud.DownloadPhotoResult, error) {
	client.t.Helper()
	//nolint:gosec // GO-15: this synthetic snapshot verifies page credential rotation before download.
	encoded, err := json.Marshal(request.Auth)
	if err != nil || !bytes.Contains(encoded, []byte("page-secret")) {
		client.t.Fatal("download did not consume the page credential rotation")
	}
	client.downloads++
	return &icloud.DownloadPhotoResult{
		Content: nullable.NewNullableWithValue([]byte("x")), Responses: syncReceipt("download"),
	}, nil
}

func syncReceipt(stage string) []icloud.ResponseMetadata {
	return []icloud.ResponseMetadata{{
		StatusCode: 200, CookieScopeURL: "https://photos.example.invalid/",
		Headers: []icloud.Header{{Name: "Set-Cookie", Value: "session=" + stage + "-secret; Path=/; Secure"}},
	}}
}

func TestPhotosSyncNativeSessionAndMaterialization(t *testing.T) {
	t.Parallel()
	client, session, request, before := syncCLISetup(t, false)
	var output bytes.Buffer
	if err := runSyncCLI(t.Context(), client, session, request, "photos-sync", "", &output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(request), "photos", "photo.jpg"))
	if err != nil || string(data) != "x" || client.downloads != 1 {
		t.Fatalf("materialization failed: %q, downloads=%d, %v", data, client.downloads, err)
	}
	checkSyncState(t, session, before, "download-secret")
	for _, secret := range []string{"-secret", "private-trust", "private-account", "responses", "accountData"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("console exposed %q", secret)
		}
	}
}

func TestPhotosSyncDiscoversNamedLibraryWithStoredAccount(t *testing.T) {
	t.Parallel()
	client, session, requestPath, before := syncCLISetup(t, false)
	client.named = true
	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	var request photosync.Request
	decode(t, data, &request)
	request.Options.Library = "selected"
	writeSyncJSON(t, requestPath, request)
	if err = runSyncCLI(t.Context(), client, session, requestPath, "photos-sync", "", io.Discard); err != nil {
		t.Fatal(err)
	}
	checkSyncState(t, session, before, "download-secret")
}

func TestPhotosWatchBoundedIterations(t *testing.T) {
	t.Parallel()
	client, session, request, before := syncCLISetup(t, true)
	var output bytes.Buffer
	if err := runSyncCLI(t.Context(), client, session, request, "photos-watch", "", &output); err != nil {
		t.Fatal(err)
	}
	if client.cursors != 2 || client.visits != 2 || strings.Count(output.String(), "\n") != 2 {
		t.Fatalf("watch did not stop at two synchronous results: cursors=%d visits=%d output=%s",
			client.cursors, client.visits, output.String())
	}
	checkSyncState(t, session, before, "page-secret")
}

type rejectedSyncOutput struct{ cause error }

func (writer rejectedSyncOutput) Write([]byte) (int, error) { return 0, writer.cause }

func TestPhotosWatchOutputFailureStopsAfterPersistingIteration(t *testing.T) {
	t.Parallel()
	client, session, request, before := syncCLISetup(t, true)
	cause := errSyncOutputRejected
	err := runSyncCLI(t.Context(), client, session, request, "photos-watch", "", rejectedSyncOutput{cause: cause})
	if !errors.Is(err, cause) || client.cursors != 1 || client.visits != 1 {
		t.Fatalf("watch continued after rejected output or lost cause: %v, cursors=%d visits=%d",
			err, client.cursors, client.visits)
	}
	checkSyncState(t, session, before, "page-secret")
}

func TestPhotosSyncCancellationPreservesReceiptAndRotatesCredentials(t *testing.T) {
	t.Parallel()
	client, session, request, before := syncCLISetup(t, false)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	client.cancel = cancel
	receipt := filepath.Join(filepath.Dir(request), "result.json")
	if err := os.WriteFile(receipt, []byte("existing-receipt"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := runSyncCLI(ctx, client, session, request, "photos-sync", receipt, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	data, readErr := os.ReadFile(receipt)
	if readErr != nil || string(data) != "existing-receipt" {
		t.Fatalf("cancelled command replaced receipt: %q %v", data, readErr)
	}
	checkSyncState(t, session, before, "download-secret")
}

func TestPhotosSyncRejectsMalformedRequestBeforeSDK(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"null", `{"auth":{},"auth":{}}`, `{"unknown":true}`, "\xff"} {
		t.Run(fmt.Sprintf("%x", input), func(t *testing.T) {
			t.Parallel()
			client, session, request, _ := syncCLISetup(t, false)
			before, err := os.ReadFile(session)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(request, []byte(input), 0o600); err != nil {
				t.Fatal(err)
			}
			if err = runSyncCLI(t.Context(), client, session, request, "photos-sync", "", io.Discard); err == nil {
				t.Fatal("malformed request accepted")
			}
			after, err := os.ReadFile(session)
			if err != nil || !bytes.Equal(before, after) || client.cursors != 0 {
				t.Fatal("invalid input modified native state or called SDK")
			}
		})
	}
}

func syncCLISetup(t *testing.T, watch bool) (*syncCLIClient, string, string, icloud.NativeAuthState) {
	t.Helper()
	sdk, err := icloud.New()
	if err != nil {
		t.Fatal(err)
	}
	client := &syncCLIClient{Client: sdk, t: t, cursors: 0, visits: 0, downloads: 0, cancel: nil, named: false}
	var state icloud.NativeAuthState
	decode(t, json.RawMessage(`{"auth":{"accountID":"stored-account","clientID":"client","photosServiceURL":"https://photos.example.invalid","headers":[]},"trustToken":"private-trust","accountName":"private-account","accountData":"e30=","accountCountryCode":"USA","challenge":{"mode":"sms","authFactors":["sms"],"authInitialRoute":"sms","hasTrustedDevices":true,"phoneNumbers":[],"securityKeyNames":[]},"codeRequested":true,"deliveryMethod":"sms","deliveryNotice":"private-notice","requiresMFA":true}`), &state)
	directory := t.TempDir()
	session, requestPath := filepath.Join(directory, "session.json"), filepath.Join(directory, "request.json")
	writeSyncJSON(t, session, state)
	request := new(photosync.Request)
	request.Auth.AccountID = "untrusted-request-account"
	request.Options = photosync.DefaultOptions(filepath.Join(directory, "photos"))
	request.Options.OnlyPrintFilenames = watch
	if watch {
		iterations := 2
		writeSyncJSON(t, requestPath, photosynccommand.PhotosWatchRequest{
			Request: *request, Watch: photosync.WatchOptions{IntervalSeconds: 1, Iterations: &iterations},
		})
	} else {
		writeSyncJSON(t, requestPath, request)
	}
	return client, session, requestPath, state
}

func writeSyncJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func runSyncCLI(ctx context.Context, client icloud.Client, session, request, operation, result string,
	output io.Writer,
) error {
	args := []string{"--session", session, "--request", request, "--timeout", "10s"}
	if result != "" {
		args = append(args, "--save-result", result)
	}
	args = append(args, operation)
	return command.RunWithInput(ctx, client, args, io.NopCloser(strings.NewReader("")),
		func(string) string { return "" }, output, io.Discard)
}

func checkSyncState(t *testing.T, path string, expected icloud.NativeAuthState, cookie string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var actual icloud.NativeAuthState
	decode(t, data, &actual)
	if !bytes.Contains(data, []byte(cookie)) {
		t.Fatalf("consumed rotation was lost: %s", data)
	}
	actual.Auth = expected.Auth
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("Photos synchronization displaced native authentication progress")
	}
}
