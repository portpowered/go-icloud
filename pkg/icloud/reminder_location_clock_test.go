package icloud_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestReminderLocationClockOrderAndParentOwnership(t *testing.T) {
	t.Parallel()

	const (
		seconds = 1700000000
		step    = time.Second / 4
	)

	clockCalls := 0

	client, err := icloud.New(icloud.WithClock(func() time.Time {
		instant := time.Unix(seconds, 0).Add(time.Duration(clockCalls) * step)
		clockCalls++

		return instant
	}), icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
		checkLocationClockPayload(t, request)

		response := new(http.Response)
		response.StatusCode = http.StatusOK
		response.Body = io.NopCloser(strings.NewReader(`{}`))
		response.Header = make(http.Header)

		return response, nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	request := locationTestRequest()
	request.Latitude, request.Longitude = 1e20, 1e-7
	request.Reminder.Modified.Set(time.Unix(1, 0).UTC())
	request.Reminder.AlarmIDs = []string{testRetainedAlarm, "keep"}
	request.Reminder.RecordChangeTag.Set("existing-revision")

	result, err := client.AddReminderLocationTrigger(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}

	checkLocationParentOwnership(t, request, result, clockCalls)
}

func checkLocationParentOwnership(t *testing.T, request icloud.AddReminderLocationTriggerRequest,
	result *icloud.AddReminderLocationTriggerResult, clockCalls int) {
	t.Helper()

	if clockCalls != 3 || result.Reminder.Modified.GetOrEmpty() != request.Reminder.Modified.GetOrEmpty() ||
		result.Reminder.RecordChangeTag.GetOrEmpty() != "existing-revision" || len(result.Reminder.AlarmIDs) != 3 {
		t.Fatal("location changed Source clock order, parent timestamp, duplicates or retained revision")
	}

	result.Reminder.AlarmIDs[0] = "mutated"
	result.Reminder.Modified.SetNull()

	if request.Reminder.AlarmIDs[0] != testRetainedAlarm || request.Reminder.Modified.IsNull() {
		t.Fatal("location result shares caller state")
	}

	if result.Trigger.Radius != 100 || result.Trigger.Proximity != icloud.ReminderArriving {
		t.Fatal("location omitted defaults differ from Source")
	}
}

func checkLocationClockPayload(t *testing.T, request *http.Request) {
	t.Helper()

	var input cloudkit.ReminderLocationRequest

	err := json.NewDecoder(request.Body).Decode(&input)
	if err != nil {
		t.Fatal(err)
	}

	parent, err := input.Operations[0].AsLocationParentOperation()
	if err != nil {
		t.Fatal(err)
	}

	alarm, err := input.Operations[1].AsLocationAlarmOperation()
	if err != nil {
		t.Fatal(err)
	}

	checkLocationCoordinatePayload(t, input.Operations[2])

	var tokens cloudkit.ReminderAlarmLinkTokensMap

	err = json.Unmarshal([]byte(parent.Record.Fields.ResolutionTokenMap.Value), &tokens)
	if err != nil {
		t.Fatal(err)
	}

	if parent.Record.Fields.LastModifiedDate.Value != 1700000000000 ||
		alarm.Record.Fields.DueDateResolutionTokenAsNonce.Value != "100721692800.25" ||
		tokens.Map.AlarmIDs.ModificationTime != testFractionalReminderTime ||
		tokens.Map.LastModifiedDate.ModificationTime != testFractionalReminderTime {
		t.Fatal("Source independent timestamp, nonce and resolution clock samples changed")
	}
}

func checkLocationCoordinatePayload(t *testing.T, operation cloudkit.ReminderLocationOperation) {
	t.Helper()

	trigger, err := operation.AsLocationTriggerOperation()
	if err != nil {
		t.Fatal(err)
	}

	if trigger.Record.Fields.Latitude.Value != "1e+20" || trigger.Record.Fields.Longitude.Value != "1e-07" {
		t.Fatal("coordinate float spelling differs from Source scientific notation")
	}
}
