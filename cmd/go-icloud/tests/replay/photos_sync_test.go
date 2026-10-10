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

const (
	syncPrivateResultFilename = "private-result.json"
	watchFilenamePattern      = "watch-%d.jpg"
	syncCursorConsoleField    = "syncCursor"
	firstPrivateSyncCursor    = "synthetic-private-cursor-1"
	secondPrivateSyncCursor   = "synthetic-private-cursor-2"
)

var errSyncOutputRejected = errors.New("synthetic output rejection")

type syncCLIClient struct {
	icloud.Client

	t           *testing.T
	cursors     int
	visits      int
	downloads   int
	cancel      context.CancelFunc
	named       bool
	watchOutput *bytes.Buffer
}

func (client *syncCLIClient) GetPhotosCursor(
	_ context.Context, request icloud.GetPhotosCursorRequest,
) (*icloud.GetPhotosCursorResult, error) {
	client.t.Helper()

	if request.Auth.AccountID != expectedReplayStoredAccount {
		client.t.Fatal("request authentication displaced the saved native account")
	}

	if client.named && (request.Library == nil || request.Library.ID != expectedReplaySelected) {
		client.t.Fatal("cursor lost the discovered library selection")
	}

	if client.watchOutput != nil && client.cursors == 1 {
		_ = checkOrderedWatchResult(client.t, client.watchOutput.Bytes(), 1)
	}

	client.cursors++

	return &icloud.GetPhotosCursorResult{
		SyncToken: fmt.Sprintf("synthetic-private-cursor-%d", client.cursors), Responses: syncReceipt("cursor"),
	}, nil
}

func (client *syncCLIClient) ListPhotoLibraries(
	_ context.Context, request icloud.ListPhotoLibrariesRequest,
) (*icloud.ListPhotoLibrariesResult, error) {
	client.t.Helper()

	if !client.named || request.Auth.AccountID != expectedReplayStoredAccount {
		client.t.Fatal("unexpected discovery or incorrect saved authentication")
	}

	var library icloud.PhotoLibrary

	decode(client.t, json.RawMessage(`{"id":"selected","zoneName":"selected-zone","zoneType":"REGULAR_CUSTOM_ZONE",`+
		`"ownerRecordName":null,"shared":false,"isSharedLibrary":false,"indexingState":"FINISHED",`+
		`"syncToken":null}`), &library)

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

	decode(client.t, json.RawMessage(`{"id":"asset","filename":"photo.jpg","masterID":"master",`+
		`"created":"2020-01-01T00:00:00Z",`+
		`"added":"2020-01-01T00:00:00Z","itemType":"image","isLivePhoto":false,"assetMetadata":{},`+
		`"dimensions":[null,null],"size":1,"versions":{"original":{"filename":"photo.jpg",`+
		`"url":"https://photos.example.invalid/photo","size":1,"type":"public.jpeg",`+
		`"checksum":"one"}}}`), &photo)

	if client.watchOutput != nil {
		photo.Filename = fmt.Sprintf(watchFilenamePattern, client.visits)
		resource := photo.Versions[expectedReplayOriginal]
		resource.Filename = photo.Filename
		photo.Versions[expectedReplayOriginal] = resource
	}

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
	if err != nil || !bytes.Contains(encoded, []byte(expectedReplayPageSecret)) {
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
		Headers: []icloud.Header{{Name: expectedSetCookieHeader, Value: "session=" + stage + "-secret; Path=/; Secure"}},
	}}
}

func TestPhotosSyncNativeSessionAndMaterialization(t *testing.T) {
	t.Parallel()
	client, session, request, before := syncCLISetup(t, false)

	var output bytes.Buffer

	receipt := filepath.Join(filepath.Dir(request), syncPrivateResultFilename)

	runErr := runSyncCLI(t.Context(), client, session, request, expectedReplayPhotosSync, receipt, &output)
	if runErr != nil {
		t.Fatal(runErr)
	}

	data, err := readSyncFile(t, filepath.Join(filepath.Dir(request), "photos", "photo.jpg"))
	if err != nil || string(data) != "x" || client.downloads != 1 {
		t.Fatalf("materialization failed: %q, downloads=%d, %v", data, client.downloads, err)
	}

	checkSyncPrivateCursor(t, receipt, output.Bytes(), firstPrivateSyncCursor, true)
	checkSyncState(t, session, before, expectedReplayDownloadSecret)

	for _, secret := range []string{
		"-secret", "private-trust", "private-account", expectedReplayResponses, expectedAccountDataKey,
	} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("console exposed %q", secret)
		}
	}
}

func TestPhotosSyncDiscoversNamedLibraryWithStoredAccount(t *testing.T) {
	t.Parallel()
	client, session, requestPath, before := syncCLISetup(t, false)
	client.named = true

	data, err := readSyncFile(t, requestPath)
	if err != nil {
		t.Fatal(err)
	}

	var request photosync.Request

	decode(t, data, &request)
	request.Options.Library = expectedReplaySelected
	writeSyncJSON(t, requestPath, request)

	err = runSyncCLI(t.Context(), client, session, requestPath, expectedReplayPhotosSync, "", io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	checkSyncState(t, session, before, expectedReplayDownloadSecret)
}

func TestPhotosWatchBoundedIterations(t *testing.T) {
	t.Parallel()
	client, session, request, before := syncCLISetup(t, true)

	var output bytes.Buffer

	client.watchOutput = &output
	receipt := filepath.Join(filepath.Dir(request), syncPrivateResultFilename)

	err := runSyncCLI(t.Context(), client, session, request, expectedReplayPhotosWatch, receipt, &output)
	if err != nil {
		t.Fatal(err)
	}

	if client.cursors != 2 || client.visits != 2 {
		t.Fatal("watch did not stop at two synchronous results")
	}

	final := checkOrderedWatchResult(t, output.Bytes(), 2)
	checkSyncState(t, session, before, expectedReplayPageSecret)
	checkSyncPrivateCursor(t, receipt, reminderCLIEncode(t, final), secondPrivateSyncCursor, false)
}

type rejectedSyncOutput struct{ cause error }

func (writer rejectedSyncOutput) Write([]byte) (int, error) { return 0, writer.cause }

func TestPhotosWatchOutputFailureStopsAfterPersistingIteration(t *testing.T) {
	t.Parallel()
	client, session, request, before := syncCLISetup(t, true)
	cause := errSyncOutputRejected

	err := runSyncCLI(t.Context(), client, session, request, expectedReplayPhotosWatch, "",
		rejectedSyncOutput{cause: cause})

	if !errors.Is(err, cause) || client.cursors != 1 || client.visits != 1 {
		t.Fatalf("watch continued after rejected output or lost cause: %v, cursors=%d visits=%d",
			err, client.cursors, client.visits)
	}

	checkSyncState(t, session, before, expectedReplayPageSecret)
}

func TestPhotosSyncCancellationPreservesReceiptAndRotatesCredentials(t *testing.T) {
	t.Parallel()
	client, session, request, before := syncCLISetup(t, false)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client.cancel = cancel

	receipt := filepath.Join(filepath.Dir(request), expectedReplayResultJSON)

	runErr := os.WriteFile(receipt, []byte(expectedReplayExistingReceipt), 0o600)
	if runErr != nil {
		t.Fatal(runErr)
	}

	err := runSyncCLI(ctx, client, session, request, expectedReplayPhotosSync, receipt, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}

	data, readErr := readSyncFile(t, receipt)
	if readErr != nil || string(data) != expectedReplayExistingReceipt {
		t.Fatalf("cancelled command replaced receipt: %q %v", data, readErr)
	}

	checkSyncState(t, session, before, expectedReplayDownloadSecret)
}

func TestPhotosSyncRejectsMalformedRequestBeforeSDK(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"null", `{"auth":{},"auth":{}}`, `{"unknown":true}`, "\xff"} {
		t.Run(fmt.Sprintf("%x", input), func(t *testing.T) {
			t.Parallel()
			client, session, request, _ := syncCLISetup(t, false)

			before, err := readSyncFile(t, session)
			if err != nil {
				t.Fatal(err)
			}

			err = os.WriteFile(request, []byte(input), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			err = runSyncCLI(t.Context(), client, session, request, expectedReplayPhotosSync, "", io.Discard)
			if err == nil {
				t.Fatal("malformed request accepted")
			}

			after, err := readSyncFile(t, session)
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

	client := &syncCLIClient{
		Client: sdk, t: t, cursors: 0, visits: 0, downloads: 0, cancel: nil, named: false, watchOutput: nil,
	}

	var state icloud.NativeAuthState

	decode(t, json.RawMessage(`{"auth":{"accountID":"stored-account","clientID":"client",`+
		`"photosServiceURL":"https://photos.example.invalid","headers":[]},`+
		`"trustToken":"private-trust","accountName":"private-account","accountData":"e30=",`+
		`"accountCountryCode":"USA","challenge":{"mode":"sms","authFactors":["sms"],`+
		`"authInitialRoute":"sms","hasTrustedDevices":true,"phoneNumbers":[],`+
		`"securityKeyNames":[]},"codeRequested":true,"deliveryMethod":"sms",`+
		`"deliveryNotice":"private-notice","requiresMFA":true}`), &state)

	directory := t.TempDir()
	session, requestPath := filepath.Join(directory, expectedReplaySessionJSON), filepath.Join(directory, "request.json")
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

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

func runSyncCLI(ctx context.Context, client icloud.Client, session, request, operation, result string,
	output io.Writer,
) error {
	args := []string{sessionFlag, session, expectedRequestOption, request, "--timeout", "10s"}
	if result != "" {
		args = append(args, expectedReplaySaveResult, result)
	}

	args = append(args, operation)

	return command.RunWithInput(ctx, client, args, io.NopCloser(strings.NewReader("")),
		func(string) string { return "" }, output, io.Discard)
}

func checkSyncState(t *testing.T, path string, expected icloud.NativeAuthState, cookie string) {
	t.Helper()

	data, err := readSyncFile(t, path)
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

func checkOrderedWatchResult(t *testing.T, output []byte, count int) photosync.Result {
	t.Helper()

	if bytes.Contains(output, []byte("synthetic-private-cursor-")) ||
		bytes.Contains(output, []byte(syncCursorConsoleField)) {
		t.Fatal("watch console exposed its private cursor")
	}

	decoder := json.NewDecoder(bytes.NewReader(output))

	var final photosync.Result

	for iteration := 1; iteration <= count; iteration++ {
		var result photosync.Result

		err := decoder.Decode(&result)
		if err != nil {
			t.Fatal(err)
		}

		if result.SyncCursor != nil || result.Library != "root" || result.ListedCount != 1 ||
			result.DownloadedCount != 0 || result.SkippedCount != 0 || result.DeletedCount != 0 ||
			result.ShortCircuited || len(result.Items) != 1 {
			t.Fatal("watch business result differs")
		}

		item := result.Items[0]
		if item.AssetID != "asset" || item.ResourceKey != expectedReplayOriginal || string(item.Action) != "listed" ||
			item.Path != fmt.Sprintf(watchFilenamePattern, iteration) ||
			item.Reason == nil || string(*item.Reason) != "print-only" {
			t.Fatal("watch result order or item differs")
		}

		final = result
	}

	var extra any

	err := decoder.Decode(&extra)
	if !errors.Is(err, io.EOF) {
		t.Fatal("watch emitted unexpected results")
	}

	return final
}

func checkSyncPrivateCursor(t *testing.T, path string, console []byte, cursor string, persisted bool) {
	t.Helper()

	data, err := readSyncFile(t, path)
	if err != nil {
		t.Fatal(err)
	}

	var saved, projected photosync.Result

	decode(t, data, &saved)
	decode(t, console, &projected)

	if saved.SyncCursor == nil || *saved.SyncCursor != cursor || projected.SyncCursor != nil ||
		bytes.Contains(console, []byte(cursor)) || bytes.Contains(console, []byte(syncCursorConsoleField)) {
		t.Fatal("private and console cursor boundaries differ")
	}

	saved.SyncCursor = nil
	if !reflect.DeepEqual(saved, projected) {
		t.Fatal("cursor redaction changed other business fields")
	}

	if persisted {
		manifestData, readErr := readSyncFile(t, saved.StatePath)
		if readErr != nil {
			t.Fatal(readErr)
		}

		var manifest photosync.Manifest

		decode(t, manifestData, &manifest)

		if manifest.Cursor == nil || *manifest.Cursor != cursor {
			t.Fatal("manifest lost the private cursor")
		}
	}
}

func readSyncFile(t *testing.T, path string) ([]byte, error) {
	t.Helper()

	directory, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open synthetic synchronization directory: %w", err)
	}

	defer func() {
		closeErr := directory.Close()
		if closeErr != nil {
			t.Errorf("close synthetic synchronization directory: %v", closeErr)
		}
	}()

	data, err := directory.ReadFile(filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("read synthetic synchronization file: %w", err)
	}

	return data, nil
}
