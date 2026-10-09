package icloud_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const (
	reminderHashtagRawID        = "synthetic"
	reminderHashtagTestName     = "Hashtag/synthetic"
	reminderHashtagRetainedID   = "keep"
	reminderHashtagMutatedValue = "mutated-relation-result"
	reminderTestOrigin          = "https://reminders.example.invalid"
)

func TestDeleteReminderHashtagRejectsWrongParentBeforeEntropy(t *testing.T) {
	t.Parallel()

	var entropy reminderFailingEntropy

	client, err := icloud.New(icloud.WithRandomSource(&entropy),
		icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
			t.Fatal("mismatched child reached transport")

			return nil, errSyntheticPeer
		})))
	if err != nil {
		t.Fatal(err)
	}

	request := new(icloud.DeleteReminderHashtagRequest)
	request.Auth = deviceRequest().Auth
	request.Reminder.ID = reminderTestRecordName
	request.Hashtag.ID = reminderHashtagTestName
	request.Hashtag.ReminderID = "another-reminder"
	result, err := client.DeleteReminderHashtag(t.Context(), *request)

	var failure *icloud.ClientError

	if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.Configuration || entropy.calls != 0 {
		t.Fatal("wrong parent was not rejected before preparation", err)
	}
}

func TestDeleteReminderHashtagNormalizesEveryLinkAndPreservesAcknowledgements(t *testing.T) {
	t.Parallel()

	client, err := icloud.New(icloud.WithRandomSource(bytes.NewReader(make([]byte, reminderDeletionEntropyBytes))),
		icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
			checkHashtagDeletionLinks(t, request)

			response := new(http.Response)
			response.StatusCode = http.StatusOK
			response.Header = make(http.Header)
			response.Body = io.NopCloser(strings.NewReader(`{"records":[` +
				`{"recordName":"Reminder/synthetic","recordType":"Reminder","fields":{},"recordChangeTag":""},` +
				`{"recordName":"Hashtag/other","recordType":"Hashtag","fields":{},"recordChangeTag":"unrelated"}]}`))

			return response, nil
		})))
	if err != nil {
		t.Fatal(err)
	}

	request := new(icloud.DeleteReminderHashtagRequest)
	request.Auth = deviceRequest().Auth
	request.Auth.RemindersServiceURL = reminderTestOrigin
	request.Reminder.ID = reminderTestRecordName
	request.Reminder.HashtagIDs = []string{reminderHashtagRawID, reminderHashtagTestName,
		"Hashtag/keep", reminderHashtagRetainedID, ""}
	request.Reminder.RecordChangeTag.Set("parent-old")
	request.Hashtag.ID = reminderHashtagRawID
	request.Hashtag.RecordChangeTag.Set("child-old")

	result, err := client.DeleteReminderHashtag(t.Context(), *request)
	if err != nil {
		t.Fatal(err)
	}

	expectedIDs := []string{reminderHashtagRetainedID, reminderHashtagRetainedID, ""}
	if !reflect.DeepEqual(result.Reminder.HashtagIDs, expectedIDs) ||
		result.Reminder.RecordChangeTag.GetOrEmpty() != "parent-old" ||
		result.Hashtag.RecordChangeTag.GetOrEmpty() != "child-old" || result.Hashtag.ID != request.Hashtag.ID {
		t.Fatal("normalization or acknowledgement preservation differs from Source")
	}

	result.Reminder.HashtagIDs[0] = reminderHashtagMutatedValue
	result.Reminder.RecordChangeTag.Set(reminderHashtagMutatedValue)
	result.Hashtag.RecordChangeTag.Set(reminderHashtagMutatedValue)

	if request.Reminder.HashtagIDs[0] != reminderHashtagRawID ||
		request.Reminder.RecordChangeTag.GetOrEmpty() != "parent-old" ||
		request.Hashtag.RecordChangeTag.GetOrEmpty() != "child-old" {
		t.Fatal("returned relation state aliases caller-owned snapshots")
	}
}

func checkHashtagDeletionLinks(t *testing.T, request *http.Request) {
	t.Helper()

	var input cloudkit.ReminderHashtagDeletionRequest

	err := json.NewDecoder(request.Body).Decode(&input)
	if err != nil {
		t.Fatal(err)
	}

	if !bool(input.Atomic) || len(input.Operations) != 2 {
		t.Fatal("deletion did not submit an atomic parent/child pair")
	}

	parent, err := input.Operations[0].AsReminderHashtagParentOperation()

	expectedIDs := []string{reminderHashtagRetainedID, reminderHashtagRetainedID, ""}

	if err != nil || !reflect.DeepEqual(parent.Record.Fields.HashtagIDs.Value, expectedIDs) {
		t.Fatal("wire ID normalization lost ordering or duplicates", err)
	}

	child, err := input.Operations[1].AsReminderHashtagDeletionOperation()
	if err != nil || child.Record.RecordName != reminderHashtagTestName || child.Record.Fields.Reminder != nil {
		t.Fatal("wire child identity or absent parent differs from Source", err)
	}
}
