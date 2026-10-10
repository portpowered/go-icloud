package photosync_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/pkg/photosync"
)

const testDownloadOperation = "download"

type sdkSourceClient struct {
	*icloud.SDK

	test    *testing.T
	visited int
	photo   icloud.Photo
}

func cookieReceipt(value string) []icloud.ResponseMetadata {
	return []icloud.ResponseMetadata{{StatusCode: 200, CookieScopeURL: "https://photos.example.invalid/service/",
		Headers: []icloud.Header{{Name: "Set-Cookie", Value: "sid=" + value + "; Path=/; Secure"}}}}
}

func assertCookie(t *testing.T, auth icloud.AuthContext, expected string) {
	t.Helper()

	for _, cookie := range auth.Cookies {
		if cookie.Name == "sid" && cookie.Value == expected {
			return
		}
	}

	t.Fatalf("expected current scoped cookie %s, got %v", expected, auth.Cookies)
}

func (client *sdkSourceClient) GetPhotosCursor(_ context.Context,
	_ icloud.GetPhotosCursorRequest,
) (*icloud.GetPhotosCursorResult, error) {
	return &icloud.GetPhotosCursorResult{SyncToken: testFirstCursor, Responses: cookieReceipt("cursor")}, nil
}

func (client *sdkSourceClient) VisitPhotoAssets(_ context.Context, input icloud.ListPhotoAssetsRequest,
	visitor icloud.PhotoVisitor,
) (*icloud.ListPhotoAssetsResult, error) {
	assertCookie(client.test, input.Auth, "cursor")

	client.visited++
	responses := cookieReceipt("page")
	_, err := visitor(icloud.PhotoVisitEvent{Photo: client.photo, Responses: responses})

	return &icloud.ListPhotoAssetsResult{Photos: []icloud.Photo{client.photo}, Responses: responses}, err
}

func (client *sdkSourceClient) DownloadPhoto(_ context.Context,
	input icloud.DownloadPhotoRequest,
) (*icloud.DownloadPhotoResult, error) {
	assertCookie(client.test, input.Auth, "page")

	result := new(icloud.DownloadPhotoResult)
	result.Content.Set([]byte("asset-1:original"))
	result.Responses = cookieReceipt(testDownloadOperation)

	return result, nil
}

func (client *sdkSourceClient) DeletePhoto(_ context.Context,
	input icloud.DeletePhotoRequest,
) (*icloud.PhotoDeletionResult, error) {
	assertCookie(client.test, input.Auth, testDownloadOperation)

	return &icloud.PhotoDeletionResult{Deleted: true, Responses: cookieReceipt("deleted")}, nil
}

func TestSDKSourceAppliesPageCookiesBeforeNestedTransfer(t *testing.T) {
	t.Parallel()

	base, err := icloud.New()
	if err != nil {
		t.Fatal(err)
	}

	asset := photo("asset-1", testPhotoFilename)
	providerPhoto := new(icloud.Photo)
	providerPhoto.ID, providerPhoto.Filename = asset.ID, asset.Filename
	providerPhoto.ItemType = icloud.Image
	providerPhoto.Created, providerPhoto.Added = *asset.TakenAt, *asset.AddedAt
	providerPhoto.AssetMetadata = json.RawMessage(`{"fields":{"assetDate":` +
		`{"value":"2026-04-01T00:00:00Z","type":"TIMESTAMP"}}}`)
	resource := new(icloud.PhotoResource)
	resource.Filename = asset.Filename
	resource.Url = json.RawMessage(`"https://photos.example.invalid/asset-1"`)
	resource.Size = json.RawMessage(`16`)
	resource.Type, resource.Checksum = json.RawMessage(`null`), json.RawMessage(`null`)
	providerPhoto.Versions = map[string]icloud.PhotoResource{testOriginalVersion: *resource}
	client := &sdkSourceClient{SDK: base, test: t, visited: 0, photo: *providerPhoto}
	input := request(t.TempDir())
	session := new(icloud.ResumeSessionResult)
	session.Auth, session.Responses = input.Auth, []icloud.ResponseMetadata{}
	session.AccountCountryCode.SetNull()

	source, err := photosync.NewSDKSource(t.Context(), client, *session, nil)
	if err != nil {
		t.Fatal(err)
	}

	days := 0
	input.Options.KeepIcloudRecentDays = &days

	result, err := newEngine(t, source).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	if result.DownloadedCount != 1 || result.DeletedCount != 1 || client.visited != 1 {
		t.Fatalf("SDK source result %+v", result)
	}

	snapshot, err := source.Snapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	assertCookie(t, snapshot.Auth, "deleted")

	if len(snapshot.Responses) != 4 {
		t.Fatalf("response receipt duplication/loss: %d", len(snapshot.Responses))
	}

	if len(session.Auth.Cookies) != 0 {
		t.Fatal("input authentication was mutated")
	}
}

func TestSDKSourceRejectsAnotherAccount(t *testing.T) {
	t.Parallel()

	client, err := icloud.New()
	if err != nil {
		t.Fatal(err)
	}

	session := new(icloud.ResumeSessionResult)
	session.Auth = request(t.TempDir()).Auth
	session.AccountCountryCode.SetNull()

	source, err := photosync.NewSDKSource(t.Context(), client, *session, nil)
	if err != nil {
		t.Fatal(err)
	}

	wrong := session.Auth
	wrong.AccountID = "other-account"
	_, err = source.Cursor(t.Context(), wrong, photosync.DefaultOptions(t.TempDir()))

	var typed *photosync.SyncError

	if !errors.As(err, &typed) {
		t.Fatalf("untyped account selection failure: %v", err)
	}
}
