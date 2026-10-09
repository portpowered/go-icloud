package replay_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestReminderZonesSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/reminders-zones-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 8 {
		t.Fatal("reminder zone scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runReminderZonesSDK(t, readAccountScenario(t, path))
		})
	}
}

func runReminderZonesSDK(t *testing.T, scenario accountScenario) {
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
	auth.RemindersServiceURL = scenario.Initial.Origin

	result, err := client.ListReminderZones(t.Context(), icloud.ListReminderZonesRequest{Auth: auth})
	if len(scenario.Error) != 0 {
		checkReminderZonesFailure(t, scenario, result, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkReminderZonesValue(t, scenario, result)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func checkReminderZonesValue(t *testing.T, scenario accountScenario, result *icloud.ListReminderZonesResult) {
	t.Helper()

	var expected cloudkit.CKZoneListResponse

	err := json.Unmarshal(scenario.Result, &expected)
	if err != nil || expected.Zones == nil || len(result.Zones) != len(*expected.Zones) {
		t.Fatal("reminder zone result shape differs from reference")
	}

	for index, zone := range *expected.Zones {
		actual := result.Zones[index]
		if actual.Name != zone.ZoneID.ZoneName || !reflect.DeepEqual(actual.Owner, zone.ZoneID.OwnerRecordName) ||
			!reflect.DeepEqual(actual.Type, zone.ZoneID.ZoneType) || !reflect.DeepEqual(actual.SyncToken, zone.SyncToken) ||
			!reflect.DeepEqual(actual.Deleted, zone.Deleted) {
			t.Fatal("complete reminder zone projection differs from reference")
		}
	}

	checkSDKMetadata(t, result.Metadata, scenario.Exchanges[0].Response)
}

func checkReminderZonesFailure(t *testing.T, scenario accountScenario,
	result *icloud.ListReminderZonesResult, err error,
) {
	t.Helper()

	var failure *icloud.ClientError

	if result != nil || !errors.As(err, &failure) {
		t.Fatal("reminder zone failure lost its typed classification")
	}

	expectedKind := icloud.Unavailable

	var referenceError map[string]string

	decodeErr := json.Unmarshal(scenario.Error, &referenceError)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	if referenceError["message"] == "Zones list response validation failed" {
		expectedKind = icloud.InvalidResponse
	}

	body := contractAuthBody(t, scenario.Exchanges[0].Response.Body)
	if failure.Kind() != expectedKind || failure.StatusCode() != scenario.Exchanges[0].Response.Status ||
		string(failure.ResponseBody()) != string(body) {
		t.Fatal("reminder zone failure lost provider evidence")
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{CookieScopeURL: failure.CookieScopeURL(),
		StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders()}, scenario.Exchanges[0].Response)
}
