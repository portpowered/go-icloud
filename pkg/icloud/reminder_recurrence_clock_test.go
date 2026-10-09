package icloud_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const recurrenceCreationEntropyBytes = 48

func TestReminderRecurrenceParentClockOrder(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{recurrenceTestCreate, recurrenceTestDelete} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()

			base := time.Unix(1700000000, 0)
			calls := 0

			client, err := icloud.New(icloud.WithRandomSource(bytes.NewReader(make([]byte, recurrenceCreationEntropyBytes))),
				icloud.WithClock(func() time.Time {
					instant := base.Add(time.Duration(calls) * time.Second)
					calls++

					return instant
				}),
				icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
					checkRecurrenceParentClock(t, request, base)

					response := new(http.Response)
					response.StatusCode = http.StatusOK
					response.Header = make(http.Header)
					response.Body = io.NopCloser(strings.NewReader(`{}`))

					return response, nil
				})))
			if err != nil {
				t.Fatal(err)
			}

			err = callRecurrenceMutation(t.Context(), client, operation)
			if err != nil {
				t.Fatal(err)
			}

			if calls != 2 {
				t.Fatal("linked recurrence did not use distinct modification and token clocks", calls)
			}
		})
	}
}

func checkRecurrenceParentClock(t *testing.T, request *http.Request, base time.Time) {
	t.Helper()

	var payload cloudkit.ReminderRecurrenceCreationRequest

	err := json.NewDecoder(request.Body).Decode(&payload)
	if err != nil {
		t.Fatal(err)
	}

	parent, err := payload.Operations[0].AsReminderRecurrenceParentOperation()
	if err != nil {
		t.Fatal(err)
	}

	fields := parent.Record.Fields
	if fields.LastModifiedDate.Value != base.UnixMilli() {
		t.Fatal("incorrect linked modification clock")
	}

	var tokens cloudkit.ReminderRecurrenceLinkTokensMap

	err = json.Unmarshal([]byte(fields.ResolutionTokenMap.Value), &tokens)
	if err != nil {
		t.Fatal(err)
	}

	want := float64(base.Add(time.Second).Unix()) - float64(cloudkit.ReminderAppleEpochUnixSeconds)

	for _, token := range []cloudkit.ReminderResolutionToken{tokens.Map.RecurrenceRuleIDs, tokens.Map.LastModifiedDate} {
		got, decodeErr := token.ModificationTime.Float64()
		if decodeErr != nil || got != want {
			t.Fatal("resolution tokens did not share second Source clock", got, want, decodeErr)
		}
	}
}
