package icloud_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestReminderMutationAdvancingClockOrder(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{reminderCreateTestOperation, reminderUpdateTestOperation,
		reminderDeleteTestOperation, reminderHashtagCreateTestOperation, reminderHashtagDeleteTestOperation} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			checkReminderAdvancingClock(t, operation)
		})
	}
}

func checkReminderAdvancingClock(t *testing.T, operation string) {
	t.Helper()

	start := time.Unix(1700000000, 0)
	reads := 0

	client, err := icloud.New(icloud.WithRandomSource(bytes.NewReader(make([]byte, 1024))),
		icloud.WithClock(func() time.Time {
			instant := start.Add(time.Duration(reads) * time.Second)
			reads++

			return instant
		}), icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
			checkReminderClockRequest(t, operation, request, start)

			response := new(http.Response)
			response.StatusCode = http.StatusServiceUnavailable
			response.Header = make(http.Header)
			response.Body = io.NopCloser(strings.NewReader("synthetic clock rejection"))

			return response, nil
		})))
	if err != nil {
		t.Fatal(err)
	}

	err = callReminderMutation(t.Context(), client, operation)

	var failure *icloud.ClientError

	wantedReads := 2
	if operation == reminderHashtagCreateTestOperation {
		wantedReads = 3
	}

	if !errors.As(err, &failure) || failure.Kind() != icloud.Unavailable || reads != wantedReads {
		t.Fatal("mutation did not preserve Source clock reads", reads, wantedReads, err)
	}
}

func checkReminderClockRequest(t *testing.T, operation string, request *http.Request, start time.Time) {
	t.Helper()

	switch operation {
	case reminderCreateTestOperation:
		var input cloudkit.ReminderCreationRequest

		decodeReminderClockRequest(t, request, &input)
		fields := input.Operations[0].Record.Fields
		checkReminderClockFields(t, fields.ResolutionTokenMap.Value,
			fields.LastModifiedDate.Value.GetOrEmpty(), start, start.Add(time.Second))
	case reminderUpdateTestOperation:
		var input cloudkit.ReminderUpdateRequest

		decodeReminderClockRequest(t, request, &input)
		fields := input.Operations[0].Record.Fields
		checkReminderClockFields(t, fields.ResolutionTokenMap.Value,
			fields.LastModifiedDate.Value.GetOrEmpty(), start.Add(time.Second), start)
	case reminderDeleteTestOperation:
		var input cloudkit.ReminderDeletionRequest

		decodeReminderClockRequest(t, request, &input)
		fields := input.Operations[0].Record.Fields
		checkReminderClockFields(t, fields.ResolutionTokenMap.Value,
			fields.LastModifiedDate.Value.GetOrEmpty(), start, start.Add(time.Second))
	default:
		checkReminderHashtagClock(t, operation, request, start)
	}
}

func decodeReminderClockRequest(t *testing.T, request *http.Request, result any) {
	t.Helper()

	err := json.NewDecoder(request.Body).Decode(result)
	if err != nil {
		t.Fatal(err)
	}
}

func checkReminderClockFields(t *testing.T, encoded string, modified int64, tokenTime, fieldTime time.Time) {
	t.Helper()

	var tokens map[string]map[string]cloudkit.ReminderResolutionToken

	err := json.Unmarshal([]byte(encoded), &tokens)
	if err != nil {
		t.Fatal(err)
	}

	if modified != fieldTime.UnixMilli() || len(tokens["map"]) == 0 {
		t.Fatal("mutation field clock differs from Source", modified, fieldTime)
	}

	for _, token := range tokens["map"] {
		seconds, err := strconv.ParseFloat(string(token.ModificationTime), 64)
		if err != nil || seconds != float64(tokenTime.Unix()-int64(cloudkit.ReminderAppleEpochUnixSeconds)) {
			t.Fatal("resolution token clock differs from Source", token.ModificationTime, tokenTime, err)
		}
	}
}

func checkReminderHashtagClock(t *testing.T, operation string, request *http.Request, start time.Time) {
	t.Helper()

	var parent cloudkit.ReminderHashtagParentOperation

	fieldTime := start

	if operation == reminderHashtagCreateTestOperation {
		var input cloudkit.ReminderHashtagCreationRequest

		decodeReminderClockRequest(t, request, &input)

		var err error

		parent, err = input.Operations[0].AsReminderHashtagParentOperation()
		if err != nil {
			t.Fatal(err)
		}

		child, err := input.Operations[1].AsReminderHashtagCreationOperation()
		if err != nil || child.Record.Fields.CreationDate.Value.GetOrEmpty() != start.UnixMilli() {
			t.Fatal("hashtag creation clock differs from Source", err)
		}

		fieldTime = start.Add(time.Second)
	} else {
		var input cloudkit.ReminderHashtagDeletionRequest

		decodeReminderClockRequest(t, request, &input)

		var err error

		parent, err = input.Operations[0].AsReminderHashtagParentOperation()
		if err != nil {
			t.Fatal(err)
		}
	}

	fields := parent.Record.Fields
	checkReminderClockFields(t, fields.ResolutionTokenMap.Value,
		fields.LastModifiedDate.Value.GetOrEmpty(), fieldTime.Add(time.Second), fieldTime)
}
