package icloud_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestDeleteReminderHashtagUnlinksEmptyIdentity(t *testing.T) {
	t.Parallel()
	client := reminderEdgeClient(t, func(request *http.Request) {
		var input cloudkit.ReminderHashtagDeletionRequest

		decodeReminderClockRequest(t, request, &input)

		parent, err := input.Operations[0].AsReminderHashtagParentOperation()
		if err != nil || !reflect.DeepEqual(parent.Record.Fields.HashtagIDs.Value, []string{reminderHashtagRetainedID}) {
			t.Fatal("empty raw or prefixed hashtag identity survived wire unlink", err)
		}
	})
	request := new(icloud.DeleteReminderHashtagRequest)
	request.Auth = deviceRequest().Auth
	request.Auth.RemindersServiceURL = reminderTestOrigin
	request.Reminder.ID = reminderTestRecordName
	request.Reminder.HashtagIDs = []string{"", "Hashtag/", reminderHashtagRetainedID}

	result, err := client.DeleteReminderHashtag(t.Context(), *request)
	if err != nil || !reflect.DeepEqual(result.Reminder.HashtagIDs, []string{reminderHashtagRetainedID}) {
		t.Fatal("empty hashtag deletion did not preserve Source unlink behavior", err)
	}

	if len(request.Reminder.HashtagIDs) != 3 {
		t.Fatal("unlink mutated caller IDs")
	}
}

func TestUpdateReminderPreservesAwareDateOffsets(t *testing.T) {
	t.Parallel()

	instant := time.Date(2024, time.March, 2, 15, 4, 5, 123000000, time.FixedZone("synthetic", -7*3600))
	client := reminderEdgeClient(t, func(request *http.Request) {
		var input cloudkit.ReminderUpdateRequest

		decodeReminderClockRequest(t, request, &input)

		fields := input.Operations[0].Record.Fields
		if fields.DueDate.Value.GetOrEmpty() != instant.UnixMilli() ||
			fields.CompletionDate.Value.GetOrEmpty() != instant.UnixMilli() {
			t.Fatal("aware date wire timestamp changed its instant")
		}
	})
	request := new(icloud.UpdateReminderRequest)
	request.Auth = deviceRequest().Auth
	request.Auth.RemindersServiceURL = reminderTestOrigin
	request.Reminder.ID = reminderTestRecordName
	request.Reminder.Completed = true
	request.Reminder.DueDate.Set(instant)
	request.Reminder.CompletedDate.Set(instant)

	result, err := client.UpdateReminder(t.Context(), *request)
	if err != nil {
		t.Fatal(err)
	}

	for _, actual := range []time.Time{result.Reminder.DueDate.GetOrEmpty(), result.Reminder.CompletedDate.GetOrEmpty()} {
		if actual.Format(time.RFC3339Nano) != instant.Format(time.RFC3339Nano) {
			t.Fatal("returned aware date offset differs from Source", actual, instant)
		}
	}
}

func reminderEdgeClient(t *testing.T, inspect func(*http.Request)) *icloud.SDK {
	t.Helper()

	client, err := icloud.New(icloud.WithRandomSource(bytes.NewReader(make([]byte, 1024))),
		icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
			inspect(request)

			response := new(http.Response)
			response.StatusCode = http.StatusOK
			response.Header = make(http.Header)
			response.Body = io.NopCloser(strings.NewReader(`{"records":[]}`))

			return response, nil
		})))
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func TestReminderMinimalSnapshotNormalizesNullableDefaults(t *testing.T) {
	t.Parallel()
	client := reminderEdgeClient(t, func(request *http.Request) {
		var input cloudkit.ReminderHashtagUpdateRequest

		decodeReminderClockRequest(t, request, &input)
	})
	request := new(icloud.UpdateReminderHashtagRequest)
	request.Auth = deviceRequest().Auth
	request.Auth.RemindersServiceURL = reminderTestOrigin

	result, err := client.UpdateReminderHashtag(t.Context(), *request)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(result.Hashtag)
	if err != nil || !result.Hashtag.Created.IsNull() || !result.Hashtag.RecordChangeTag.IsNull() {
		t.Fatal("minimal hashtag snapshot omitted required nullable defaults", string(encoded), err)
	}
}
