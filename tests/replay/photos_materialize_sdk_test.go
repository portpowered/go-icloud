package replay_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	materializedPhotoPath   = "2024/01/02/synthetic-0.JPG"
	materializedSyncCursor  = "synthetic-sync"
	materializedResourceKey = "original"
)

type materializationCase struct {
	Name      string          `json:"name"`
	InputHex  string          `json:"inputHex"`
	OutputHex string          `json:"outputHex"`
	Records   json.RawMessage `json:"records"`
	XMP       string          `json:"xmp"`
}

type materializationRecording struct {
	Provenance     json.RawMessage       `json:"provenance"`
	TargetIdentity json.RawMessage       `json:"targetIdentity"`
	Cases          []materializationCase `json:"cases"`
}

// LIB-05: the complete SDK traffic precedes comparison with independent Source files.
func TestPinnedSourceSDKMaterializationReplay(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("fixtures/synthetic/filesystem/photos-materialize-sdk.json")
	if err != nil {
		t.Fatal(err)
	}

	recording := new(materializationRecording)
	authReplayDecode(t, data, recording)

	if len(recording.Provenance) == 0 || len(recording.Cases) == 0 {
		t.Fatal("pinned Source materialization pairs are absent")
	}

	for _, variant := range recording.Cases {
		t.Run(variant.Name, func(t *testing.T) {
			t.Parallel()

			replaySDKMaterialization(t, variant, recording.TargetIdentity)
		})
	}
}

func replaySDKMaterialization(t *testing.T, variant materializationCase, targetIdentity json.RawMessage) {
	t.Helper()

	exchanges, auth := materializationTraffic(t, variant)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	session := new(icloud.ResumeSessionResult)
	session.Auth, session.Responses = auth, []icloud.ResponseMetadata{}

	provider, err := photosync.NewSDKSource(t.Context(), client, *session, nil)
	if err != nil {
		t.Fatal(err)
	}

	engine := newEngine(t, provider)
	input := request(filepath.Join(t.TempDir(), "output"))
	input.Auth = auth
	input.Options.SetExifDatetime, input.Options.XmpSidecar = true, true
	input.Options.FolderStructure = "%Y/%m/%d"
	key := materializationTargetKey(t, targetIdentity, input.Options.Directory)

	first, err := engine.Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	assertMaterializationResult(t, input, first, key, false)
	assertMaterializationFiles(t, input.Options.Directory, variant)
	manifest := assertMaterializationManifest(t, input.Options.Directory, key, variant)

	second, err := engine.Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	assertMaterializationResult(t, input, second, key, true)
	assertMaterializationFiles(t, input.Options.Directory, variant)

	repeated := assertMaterializationManifest(t, input.Options.Directory, key, variant)
	if !reflect.DeepEqual(repeated, manifest) {
		t.Fatal("repeat sync changed provider identity or persisted materialized size")
	}

	snapshot, err := provider.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	checkReminderSyncResponses(t, snapshot.Responses, exchanges)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func materializationTraffic(t *testing.T, variant materializationCase) ([]replay.Exchange, icloud.AuthContext) {
	t.Helper()

	cursor := readAccountScenario(t, "fixtures/synthetic/http/photos-sync-cached.json")
	assets := readAccountScenario(t, "fixtures/synthetic/http/photos-assets-1.json")
	download := readAccountScenario(t, "fixtures/synthetic/http/photos-download-binary.json")

	input, err := hex.DecodeString(variant.InputHex)
	if err != nil {
		t.Fatal(err)
	}

	// Keep the independently recorded full request boundaries; only fixture-declared
	// response records and media vary between these synthetic Source input pairs.
	materializationEntity(t, &assets.Exchanges[len(assets.Exchanges)-1].Response.Body, variant.Records)
	materializationEntity(t, &download.Exchanges[len(download.Exchanges)-2].Response.Body, variant.Records)
	materializationEntity(t, &download.Exchanges[len(download.Exchanges)-1].Response.Body, input)
	first := append([]replay.Exchange{}, cursor.Exchanges...)
	first = append(first, assets.Exchanges[1:]...)
	first = append(first, download.Exchanges[1:]...)
	first = append(first, assets.Exchanges[1:]...)
	auth := sdkAccountAuth(cursor.Initial)
	auth.PhotosServiceURL = cursor.Initial.Origin

	return first, auth
}

func materializationEntity(t *testing.T, entity *replay.Entity, data []byte) {
	t.Helper()

	value, err := json.Marshal(base64.StdEncoding.EncodeToString(data))
	if err != nil {
		t.Fatal(err)
	}

	entity.Value = value
}

func assertMaterializationResult(t *testing.T, input photosync.Request,
	actual *photosync.Result, key string, repeated bool,
) {
	t.Helper()

	want := new(photosync.Result)
	want.Albums, want.Library, want.Directory = []string{}, input.Options.Library, input.Options.Directory
	want.StatePath = filepath.Join(input.Options.Directory, ".go-icloud-state", key+".json")
	cursor := materializedSyncCursor
	want.SyncCursor = &cursor
	item := photosync.Item{AssetID: "synthetic-asset-0", ResourceKey: materializedResourceKey, Path: materializedPhotoPath,
		Action: photosync.Downloaded, Reason: nil}
	want.DownloadedCount = 1

	if repeated {
		reason := photosync.AlreadyCurrent
		item.Action, item.Reason = photosync.Skipped, &reason
		want.DownloadedCount, want.SkippedCount = 0, 1
	}

	want.Items = []photosync.Item{item}
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("complete materialization result: got %+v want %+v", actual, want)
	}
}

func assertMaterializationFiles(t *testing.T, directory string, variant materializationCase) {
	t.Helper()

	root := materializationRoot(t, directory)
	data := materializationRead(t, root, filepath.FromSlash(materializedPhotoPath))

	if hex.EncodeToString(data) != variant.OutputHex {
		t.Fatalf("JPEG differs from complete pinned Source output: %x", data)
	}

	xmp := materializationRead(t, root, filepath.FromSlash(materializedPhotoPath)+".xmp")

	if string(xmp) != variant.XMP {
		t.Fatalf("sidecar differs from complete pinned Source output: %s", xmp)
	}
}

func assertMaterializationManifest(t *testing.T, directory, key string,
	variant materializationCase,
) photosync.Manifest {
	t.Helper()

	root := materializationRoot(t, directory)
	data := materializationRead(t, root, filepath.Join(".go-icloud-state", key+".json"))

	manifest := new(photosync.Manifest)
	authReplayDecode(t, data, manifest)

	providerSize, localSize := int64(len(variant.InputHex)/2), int64(len(variant.OutputHex)/2)
	downloaded := time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC)
	cursor := materializedSyncCursor

	want := photosync.Manifest{TargetKey: key, Cursor: &cursor,
		Resources: []photosync.SyncedResource{{AssetID: "synthetic-asset-0", ResourceKey: materializedResourceKey,
			RelativePath: materializedPhotoPath, Size: &providerSize, LocalSize: &localSize,
			Checksum: nil, DownloadedAt: &downloaded}}}
	if !reflect.DeepEqual(*manifest, want) {
		t.Fatalf("complete materialization manifest: got %+v want %+v", *manifest, want)
	}

	return *manifest
}

func materializationTargetKey(t *testing.T, identity json.RawMessage, directory string) string {
	t.Helper()

	encoded, err := json.Marshal(directory)
	if err != nil {
		t.Fatal(err)
	}

	placeholder := []byte(`"$DESTINATION"`)
	if bytes.Count(identity, placeholder) != 1 {
		t.Fatal("fixture must declare exactly one directory relocation")
	}

	relocated := bytes.ReplaceAll(identity, placeholder, encoded)

	var canonical bytes.Buffer

	err = json.Compact(&canonical, relocated)
	if err != nil {
		t.Fatal(err)
	}

	// The fixture fixes contract values and JSON order independently of Options,
	// TargetIdentity's generated struct, and the production identity helper.
	digest := sha256.Sum256(canonical.Bytes())

	return hex.EncodeToString(digest[:])
}

func materializationRoot(t *testing.T, directory string) *os.Root {
	t.Helper()

	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		err := root.Close()
		if err != nil {
			t.Error(err)
		}
	})

	return root
}

func materializationRead(t *testing.T, root *os.Root, relative string) []byte {
	t.Helper()

	data, err := root.ReadFile(relative)
	if err != nil {
		t.Fatal(err)
	}

	return data
}
