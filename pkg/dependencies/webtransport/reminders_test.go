package webtransport_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
)

func TestReminderZoneResponseRequiredIdentities(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`null`, `[]`, `{"zones":null}`, `{"zones":{}}`, `{"zones":[{}]}`,
		`{"zones":[{"zoneID":null}]}`, `{"zones":[{"zoneID":{}}]}`,
		`{"zones":[{"zoneID":{"zoneName":null}}]}`,
		`{"zones":[{"zoneID":{"zoneName":1}}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			_, err := reminderZoneResponse(t, body)

			var failure *webtransport.ResponseError

			if !errors.As(err, &failure) || failure.Stage != webtransport.Decode {
				t.Fatal("invalid provider zone identity was accepted")
			}
		})
	}
}

func TestReminderZoneResponseDefaultAndEmptyLists(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`{}`, `{"zones":[]}`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			data, err := reminderZoneResponse(t, body)
			if err != nil || (data.Data.Zones != nil && len(*data.Data.Zones) != 0) {
				t.Fatal("empty/default provider zone list was rejected")
			}
		})
	}
}

func reminderZoneResponse(t *testing.T, body string) (*webtransport.ReminderZonesResponse, error) {
	t.Helper()

	response := new(http.Response)
	response.StatusCode = http.StatusOK
	response.Header = make(http.Header)
	response.Body = io.NopCloser(strings.NewReader(body))
	client := webtransport.New(findMyRoundTrip(func(_ *http.Request) (*http.Response, error) { return response, nil }))
	auth := new(webtransport.RequestContext)
	auth.Origin = "https://reminders.example.invalid"
	auth.Headers = make(http.Header)
	auth.Params.ClientId = "synthetic-client"
	auth.Params.Dsid = "synthetic-account"

	result, err := client.ListReminderZones(t.Context(), *auth)
	if err != nil {
		return nil, fmt.Errorf("synthetic reminder zone response: %w", err)
	}

	return result, nil
}
