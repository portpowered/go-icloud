package replay_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoChangesSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-*changes*.json")
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		scenario := readAccountScenario(t, path)
		if scenario.Operation != "iter_changes" {
			continue
		}

		t.Run(filepath.Base(path), func(t *testing.T) { t.Parallel(); runPhotoChangesSDK(t, path, scenario) })
	}
}

func photoChangesInputs(t *testing.T, path string) (string, *string, *string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage

	err = json.Unmarshal(data, &fields)
	if err != nil {
		t.Fatal(err)
	}

	var (
		library  string
		keywords map[string]*string
		cursor   *string
	)

	if raw := fields["library"]; raw != nil {
		err = json.Unmarshal(raw, &library)
		if err != nil {
			t.Fatalf(replayCursorErrorsFormat, err, errors.Unwrap(err), errors.Unwrap(errors.Unwrap(err)))
		}
	}

	if raw := fields[replayLiteralKeywordInputs]; raw != nil {
		err = json.Unmarshal(raw, &keywords)
		if err != nil {
			t.Fatalf(replayCursorErrorsFormat, err, errors.Unwrap(err), errors.Unwrap(errors.Unwrap(err)))
		}
	}

	raw := fields["photos_sync_token"]
	if raw == nil {
		t.Fatal("missing independently observed Source cursor")
	}

	err = json.Unmarshal(raw, &cursor)
	if err != nil {
		t.Fatal(err)
	}

	return library, keywords["since"], cursor
}

func runPhotoChangesSDK(t *testing.T, path string, scenario accountScenario) {
	t.Helper()

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
	key, since, cursor := photoChangesInputs(t, path)

	library, prior := selectedPhotoChangesLibrary(t, client, auth, key)

	actual, err := client.GetPhotoChanges(
		t.Context(),
		icloud.GetPhotoChangesRequest{Auth: auth, Library: library, Since: since},
	)
	if len(scenario.Error) != 0 {
		if actual != nil {
			t.Fatal("partial photo changes on failure")
		}

		checkSelectedPhotoFailure(t, scenario, prior, err, icloud.Unavailable)
	} else {
		if err != nil {
			t.Fatalf(replayCursorErrorsFormat, err, errors.Unwrap(err), errors.Unwrap(errors.Unwrap(err)))
		}

		checkPhotoChangesProjection(t, actual.Changes, scenario.Result)
		checkReminderSyncResponses(t, append(prior, actual.Responses...), scenario.Exchanges)
		checkPhotoChangesCursor(t, actual, cursor)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkPhotoChangesCursor(t *testing.T, actual *icloud.GetPhotoChangesResult, expected *string) {
	t.Helper()

	if expected == nil {
		if !actual.SyncToken.IsNull() {
			t.Fatal("photo changes returned a cursor where Source observed null")
		}

		return
	}

	token, err := actual.SyncToken.Get()
	if err != nil || token != *expected {
		t.Fatalf("photo changes cursor = %q (%v), Source observed %q", token, err, *expected)
	}
}

func TestPhotosCursorSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"photos-sync-cached.json", "photos-sync-zone.json", "photos-sync-missing.json"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			scenario := readAccountScenario(t, filepath.Join(replayExpectedFixturesSyntheticHTTP, name))

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

			actual, err := client.GetPhotosCursor(t.Context(), icloud.GetPhotosCursorRequest{Auth: auth, Library: nil})
			if len(scenario.Error) != 0 {
				if actual != nil {
					t.Fatal("cursor returned on failure")
				}

				checkSelectedPhotoFailure(t, scenario, nil, err, icloud.Unavailable)
			} else {
				if err != nil {
					t.Fatal(err)
				}

				checkSDKValue(t, actual.SyncToken, scenario.Result)
				checkReminderSyncResponses(t, actual.Responses, scenario.Exchanges)
			}

			err = transport.AssertConsumed()
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func checkSelectedPhotoFailure(
	t *testing.T,
	scenario accountScenario,
	prior []icloud.ResponseMetadata,
	err error,
	kind icloud.ErrorKind,
) {
	t.Helper()

	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("selected photo failure class: %v", err)
	}

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	if string(failure.ResponseBody()) != string(contractAuthBody(t, last.Body)) {
		t.Fatal("selected photo failure body differs")
	}

	checkSDKMetadata(
		t,
		icloud.ResponseMetadata{
			StatusCode:     failure.StatusCode(),
			Headers:        failure.ResponseHeaders(),
			CookieScopeURL: failure.CookieScopeURL(),
		},
		last,
	)
	checkReminderSyncResponses(
		t,
		append(prior, failure.PriorResponses()...),
		scenario.Exchanges[:len(scenario.Exchanges)-1],
	)
}

func checkPhotoChangesProjection(t *testing.T, actual []icloud.PhotoChange, raw json.RawMessage) {
	t.Helper()

	var expected []map[string]json.RawMessage

	err := json.Unmarshal(raw, &expected)
	if err != nil {
		t.Fatalf(replayCursorErrorsFormat, err, errors.Unwrap(err), errors.Unwrap(errors.Unwrap(err)))
	}

	for _, change := range expected {
		change["recordName"] = change[replayLiteralRecordName]
		delete(change, replayLiteralRecordName)
		change["recordType"] = change[replayLiteralRecordType]
		delete(change, replayLiteralRecordType)

		if string(change[replayLiteralModified]) != "null" {
			var value string

			decodeErr := json.Unmarshal(change[replayLiteralModified], &value)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}

			instant, parseErr := time.Parse(time.RFC3339Nano, value)
			if parseErr != nil {
				t.Fatal(parseErr)
			}

			change[replayLiteralModified], err = json.Marshal(instant.UTC())
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	encoded, encodeErr := json.Marshal(expected)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}

	checkSDKValue(t, actual, encoded)
}

func selectedPhotoChangesLibrary(
	t *testing.T,
	client *icloud.SDK,
	auth icloud.AuthContext,
	key string,
) (*icloud.PhotoLibrary, []icloud.ResponseMetadata) {
	t.Helper()

	if key == "" {
		return nil, []icloud.ResponseMetadata{}
	}

	libraries, err := client.ListPhotoLibraries(t.Context(), icloud.ListPhotoLibrariesRequest{Auth: auth})
	if err != nil {
		t.Fatal(err)
	}

	for _, library := range libraries.Libraries {
		if library.ID == key {
			return &library, libraries.Responses
		}
	}

	t.Fatal("selected library missing")

	return nil, nil
}
