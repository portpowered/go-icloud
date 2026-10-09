package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoDownloadSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-download-*.json")
	if err != nil || len(paths) != 25 {
		t.Fatal("photo download inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runPhotoDownloadSDK(t, path)
		})
	}
}

func runPhotoDownloadSDK(t *testing.T, path string) {
	t.Helper()
	scenario := readAccountScenario(t, path)
	row := authReplayObject(t, path)

	var inputs []string

	authReplayDecode(t, row["inputs"], &inputs)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	request := icloud.DownloadPhotoRequest{Auth: auth, Album: photoCountFixtureAlbum(t, path), PhotoID: inputs[0],
		Version: nil}

	if len(row["version"]) != 0 {
		var version icloud.PhotoVersion

		authReplayDecode(t, row["version"], &version)
		request.Version = &version
	}

	result, err := client.DownloadPhoto(t.Context(), request)
	if len(scenario.Error) != 0 {
		checkPhotoDownloadFailure(t, scenario, result, err)
	} else {
		checkPhotoDownloadResult(t, scenario, result, err)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkPhotoDownloadResult(t *testing.T, scenario accountScenario, result *icloud.DownloadPhotoResult, err error) {
	t.Helper()

	if err != nil || result == nil {
		t.Fatal("photo download failed", err)
	}

	if string(scenario.Result) == reminderChangeNullValue {
		if !result.Content.IsNull() {
			t.Fatal("unavailable rendition was not explicit null")
		}
	} else {
		var expected string

		authReplayDecode(t, scenario.Result, &expected)
		data, decodeErr := base64.StdEncoding.DecodeString(expected)

		actual, contentErr := result.Content.Get()

		if decodeErr != nil || contentErr != nil || !bytes.Equal(actual, data) || result.Content.IsNull() {
			t.Fatal("exact downloaded bytes or empty/null distinction changed", decodeErr, contentErr)
		}

		var encoded map[string]json.RawMessage

		value, encodeErr := json.Marshal(result)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}

		authReplayDecode(t, value, &encoded)
		checkSDKValue(t, encoded["content"], scenario.Result)
	}

	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
}

func checkPhotoDownloadFailure(t *testing.T, scenario accountScenario, result *icloud.DownloadPhotoResult, err error) {
	t.Helper()

	if result != nil {
		t.Fatal("photo download returned partial output")
	}

	last := scenario.Exchanges[len(scenario.Exchanges)-1]
	if last.Error == "" {
		checkPhotoDownloadProviderFailure(t, scenario, err)

		return
	}

	var failure *icloud.ClientError

	if !errors.As(err, &failure) || failure.Kind() != icloud.Transport || failure.StatusCode() != 0 ||
		len(failure.ResponseBody()) != 0 || len(failure.ResponseHeaders()) != 0 || failure.CookieScopeURL() != "" {
		t.Fatal("photo download lost transport failure classification or fabricated response evidence", err)
	}

	checkReminderSyncResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}

func checkPhotoDownloadProviderFailure(t *testing.T, scenario accountScenario, err error) {
	t.Helper()

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response

	kind := reminderSyncFailureKind(last.Status)

	if last.Status == 404 {
		kind = icloud.NotFound
	}

	var failure *icloud.ClientError

	if !errors.As(err, &failure) || failure.Kind() != kind ||
		!bytes.Equal(failure.ResponseBody(), contractAuthBody(t, last.Body)) {
		t.Fatal("photo download provider failure class or exact body changed", err)
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		CookieScopeURL: failure.CookieScopeURL()}, last)
	checkReminderSyncResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}
