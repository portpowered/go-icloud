package replay_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/accountapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotosContainerSharedLookup(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-container-shared-lookup-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 5 {
		t.Fatal("shared lookup inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) { t.Parallel(); runPhotosContainerLookup(t, path) })
	}
}
func runPhotosContainerLookup(t *testing.T, path string) {
	t.Helper()
	scenario := readAccountScenario(t, path)

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	params := accountParameters[accountapi.ListAccountDevicesParams](t, scenario.Initial)

	headers := http.Header{}
	for key, value := range scenario.Initial.Headers {
		headers.Set(key, value)
	}

	keywords := photoScenarioKeywords(t, path)

	var (
		names []string
		zone  cloudkit.CKZoneIDReq
	)

	authReplayDecode(t, keywords["record_names"], &names)
	authReplayDecode(t, keywords["zone_id"], &zone)

	auth := webtransport.RequestContext{
		PhotoZone:   nil,
		PhotoShared: false,
		DriveToken:  "",
		Cookies:     nil,
		Origin:      scenario.Initial.Origin,
		Params:      *params,
		Headers:     headers,
	}
	boundary := webtransport.New(transport)

	initial, err := boundary.PhotosIndexing(t.Context(), auth)
	if err != nil {
		t.Fatal(err)
	}

	checkPhotosContainerMetadata(t, initial.Metadata, scenario.Exchanges[0])

	auth.PhotoShared = true
	auth.PhotoZone = &zone
	result, err := boundary.LookupPhotosRecords(t.Context(), auth, names)

	checkPhotosContainerLookup(t, scenario, result, err)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

// Pydantic model_dump includes absent nullable defaults and an empty pluginFields map.
// Add only those defaults; every observed field and nested value still compares exactly.
func applySourceNullDefaults(actual, expected any) {
	switch want := expected.(type) {
	case map[string]any:
		got, ok := actual.(map[string]any)
		if !ok {
			return
		}

		for key, value := range want {
			current, exists := got[key]
			if !exists && sourceLookupDefault(key, value) {
				got[key] = value
			} else {
				applySourceNullDefaults(current, value)
			}
		}
	case []any:
		got, ok := actual.([]any)
		if !ok || len(got) != len(want) {
			return
		}

		for index, value := range want {
			applySourceNullDefaults(got[index], value)
		}
	}
}

func checkPhotosContainerLookup(
	t *testing.T,
	scenario accountScenario,
	result *webtransport.ReminderLookupResponse,
	err error,
) {
	t.Helper()

	if len(scenario.Error) != 0 {
		var failure *webtransport.ResponseError
		if result != nil || !errors.As(err, &failure) {
			t.Fatalf("lookup failure: %v", err)
		}

		checkPhotosContainerMetadata(
			t,
			&webtransport.BytesResponse{
				Status:         failure.Status,
				Body:           failure.Body,
				Headers:        failure.Headers,
				CookieScopeURL: failure.CookieScopeURL,
			},
			scenario.Exchanges[len(scenario.Exchanges)-1],
		)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	checkPhotosContainerMetadata(t, result.Metadata, scenario.Exchanges[len(scenario.Exchanges)-1])
	{
		actual, encodeErr := json.Marshal(result.Data)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}

		var got, want any

		authReplayDecode(t, actual, &got)
		authReplayDecode(t, scenario.Result, &want)
		applySourceNullDefaults(got, want)

		encoded, encodeErr := json.Marshal(got)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}

		checkSDKValue(t, json.RawMessage(encoded), scenario.Result)
	}
}

func sourceLookupDefault(key string, value any) bool {
	if value == nil {
		return true
	}

	fields, ok := value.(map[string]any)

	return key == replayExpectedPluginFields && ok && len(fields) == 0
}

func checkPhotosContainerMetadata(t *testing.T, actual *webtransport.BytesResponse, exchange replay.Exchange) {
	t.Helper()

	expected := http.Header{}
	for _, pair := range exchange.Response.Headers {
		expected.Add(pair[0], pair[1])
	}

	if actual.Status != exchange.Response.Status || !reflect.DeepEqual(actual.Headers, expected) ||
		string(actual.Body) != string(contractAuthBody(t, exchange.Response.Body)) ||
		actual.CookieScopeURL != exchange.Request.Origin+exchange.Request.Path {
		t.Fatal("Photos container response body/header/status/cookie scope differs")
	}
}
