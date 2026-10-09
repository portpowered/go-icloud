package icloud_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const reminderLocationTestOrigin = "https://reminders.example.invalid"

func TestReminderLocationPreparationFailures(t *testing.T) {
	t.Parallel()

	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "entropy", true: "canceled"}[canceled], func(t *testing.T) {
			t.Parallel()

			var entropy reminderFailingEntropy

			client, err := icloud.New(icloud.WithRandomSource(&entropy),
				icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
					t.Fatal("location preparation reached HTTP")

					return nil, errSyntheticPeer
				})))
			if err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			kind, cause, reads := icloud.Configuration, io.ErrUnexpectedEOF, 1

			if canceled {
				cancel()

				kind, cause, reads = icloud.Canceled, context.Canceled, 0
			}

			_, err = client.AddReminderLocationTrigger(ctx, locationTestRequest())

			var failure *icloud.ClientError

			if !errors.As(err, &failure) || failure.Kind() != kind || !errors.Is(err, cause) || entropy.calls != reads {
				t.Fatal("incorrect location preparation failure", err)
			}
		})
	}
}

func TestReminderLocationRejectsInvalidGeometryAfterIdentities(t *testing.T) {
	t.Parallel()

	for _, invalidRadius := range []bool{false, true} {
		t.Run(map[bool]string{false: "proximity", true: "radius"}[invalidRadius], func(t *testing.T) {
			t.Parallel()

			const entropySize = 3 * 16

			entropy := bytes.NewReader(make([]byte, entropySize))

			client, err := icloud.New(icloud.WithRandomSource(entropy),
				icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
					t.Fatal("invalid geometry reached HTTP")

					return nil, errSyntheticPeer
				})))
			if err != nil {
				t.Fatal(err)
			}

			request := locationTestRequest()

			radius, proximity := -1.0, icloud.AddReminderLocationTriggerRequestProximity(0)

			if invalidRadius {
				request.Radius = &radius
			} else {
				request.Proximity = &proximity
			}

			result, err := client.AddReminderLocationTrigger(t.Context(), request)

			var failure *icloud.ClientError

			if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.Configuration || entropy.Len() != 0 {
				t.Fatal("invalid geometry was accepted or entropy order changed", err)
			}
		})
	}
}

func locationTestRequest() icloud.AddReminderLocationTriggerRequest {
	request := new(icloud.AddReminderLocationTriggerRequest)
	request.Auth = deviceRequest().Auth
	request.Auth.RemindersServiceURL = reminderLocationTestOrigin
	request.Reminder.ID = reminderTestRecordName

	return *request
}

func TestReminderLocationFailureEvidence(t *testing.T) {
	t.Parallel()

	for name, row := range map[string]struct {
		body string
		kind icloud.ErrorKind
	}{"invalid-json": {body: "not JSON", kind: icloud.InvalidResponse},
		"invalid-record": {body: `{"records":[false]}`, kind: icloud.InvalidResponse},
		"record-rejection": {body: `{"records":[{"recordName":"Alarm/synthetic","serverErrorCode":"CONFLICT"}]}`,
			kind: icloud.Provider}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := sdkForResponse(t, http.StatusOK, row.body)
			result, err := client.AddReminderLocationTrigger(t.Context(), locationTestRequest())

			var failure *icloud.ClientError

			if result != nil || !errors.As(err, &failure) || failure.Kind() != row.kind ||
				failure.StatusCode() != http.StatusOK || string(failure.ResponseBody()) != row.body {
				t.Fatal("location failure lost provider evidence", err)
			}

			if len(failure.ResponseHeaders()) == 0 || failure.CookieScopeURL() == "" {
				t.Fatal("location failure lost headers or cookie scope")
			}
		})
	}
}
