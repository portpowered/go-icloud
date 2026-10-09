package replay_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotosDatabaseChangesSDK(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/photos-container-*-changes-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 10 {
		t.Fatal("database change inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) { t.Parallel(); runPhotosDatabaseSDK(t, path) })
	}
}
func photoScenarioKeywords(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage

	authReplayDecode(t, data, &fields)

	var keywords map[string]json.RawMessage
	if len(fields["keyword_inputs"]) != 0 {
		authReplayDecode(t, fields["keyword_inputs"], &keywords)
	}

	return keywords
}
func runPhotosDatabaseSDK(t *testing.T, path string) {
	t.Helper()
	scenario := readAccountScenario(t, path)

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
	keywords := photoScenarioKeywords(t, path)

	var since *string

	if raw := keywords["sync_token"]; raw != nil {
		authReplayDecode(t, raw, &since)
	}

	result, err := client.GetPhotoLibraryChanges(
		t.Context(),
		icloud.GetPhotoLibraryChangesRequest{
			Auth:   auth,
			Shared: strings.Contains(filepath.Base(path), "-shared-"),
			Since:  since,
		},
	)

	if len(scenario.Error) != 0 {
		kind := icloud.InvalidResponse
		if scenario.Exchanges[len(scenario.Exchanges)-1].Response.Status >= 400 {
			kind = icloud.Unavailable
		}

		checkSelectedPhotoFailure(t, scenario, nil, err, kind)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkPhotoDatabaseProjection(t, result, scenario.Result)
		checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)
	}

	err = transport.AssertConsumed()

	if err != nil {
		t.Fatal(err)
	}
}
func checkPhotoDatabaseProjection(t *testing.T, result *icloud.GetPhotoLibraryChangesResult, raw json.RawMessage) {
	t.Helper()

	var fields map[string]json.RawMessage

	authReplayDecode(t, raw, &fields)

	var zones []map[string]json.RawMessage

	authReplayDecode(t, fields["zones"], &zones)

	for _, zone := range zones {
		var identity map[string]json.RawMessage

		authReplayDecode(t, zone["zoneID"], &identity)
		delete(zone, "zoneID")

		maps.Copy(zone, identity)
	}

	encoded, err := json.Marshal(zones)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, result.Zones, encoded)
	checkSDKValue(t, result.SyncToken, fields["syncToken"])
	checkSDKValue(t, result.MoreComing, fields["moreComing"])
}
