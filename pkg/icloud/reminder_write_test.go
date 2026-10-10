package icloud_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const (
	reminderTestRecordName             = "Reminder/synthetic"
	reminderCreateTestOperation        = "reminder-create"
	reminderUpdateTestOperation        = "reminder-update"
	reminderDeleteTestOperation        = "reminder-delete"
	reminderHashtagCreateTestOperation = "hashtag-create"
	reminderHashtagUpdateTestOperation = "hashtag-update"
	reminderHashtagDeleteTestOperation = "hashtag-delete"
)

type reminderFailingEntropy struct{ calls int }

func (reader *reminderFailingEntropy) Read(_ []byte) (int, error) {
	reader.calls++

	return 0, io.ErrUnexpectedEOF
}

func TestReminderWritesCancellationAndEntropyFailure(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{reminderCreateTestOperation,
		reminderUpdateTestOperation, reminderDeleteTestOperation, reminderHashtagCreateTestOperation,
		reminderHashtagUpdateTestOperation, reminderHashtagDeleteTestOperation} {
		for _, canceled := range []bool{false, true} {
			if operation == reminderHashtagUpdateTestOperation && !canceled {
				continue
			}

			t.Run(operation+map[bool]string{false: testExpectedEntropy, true: testCanceledSuffix}[canceled], func(t *testing.T) {
				t.Parallel()
				checkReminderPreparationFailure(t, operation, canceled)
			})
		}
	}
}

func checkReminderPreparationFailure(t *testing.T, operation string, canceled bool) {
	t.Helper()

	var entropy reminderFailingEntropy

	client, err := icloud.New(icloud.WithRandomSource(&entropy),
		icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
			t.Fatal("failed preparation reached the transport")

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

	err = callReminderMutation(ctx, client, operation)

	var failure *icloud.ClientError

	if !errors.As(err, &failure) || failure.Kind() != kind || !errors.Is(err, cause) {
		t.Fatal("incorrect preparation failure", err)
	}

	if entropy.calls != reads {
		t.Fatal("preparation consumed unexpected entropy")
	}
}

//nolint:wrapcheck // LIB-05: return the client failure unchanged for exact failure assertions.
func callReminderMutation(ctx context.Context, client *icloud.SDK, operation string) error {
	auth := deviceRequest().Auth
	auth.RemindersServiceURL = reminderTestOrigin

	switch operation {
	case reminderHashtagCreateTestOperation, reminderHashtagUpdateTestOperation, reminderHashtagDeleteTestOperation:
		return callReminderHashtagMutation(ctx, client, auth, operation)
	case reminderCreateTestOperation:
		request := new(icloud.CreateReminderRequest)
		request.Auth = auth
		request.Title = "synthetic-reminder-title"
		request.ListID = "List/synthetic"
		_, err := client.CreateReminder(ctx, *request)

		return err
	case reminderUpdateTestOperation:
		request := new(icloud.UpdateReminderRequest)
		request.Auth = auth
		request.Reminder.ID = reminderTestRecordName
		_, err := client.UpdateReminder(ctx, *request)

		return err
	default:
		request := icloud.DeleteReminderRequest{Auth: auth, ReminderID: reminderTestRecordName, RecordChangeTag: nil}
		_, err := client.DeleteReminder(ctx, request)

		return err
	}
}

//nolint:wrapcheck // LIB-05: preserve client failures for exact assertions.
func callReminderHashtagMutation(ctx context.Context, client *icloud.SDK,
	auth icloud.AuthContext, operation string,
) error {
	switch operation {
	case reminderHashtagCreateTestOperation:
		request := new(icloud.CreateReminderHashtagRequest)
		request.Auth = auth
		request.Reminder.ID = reminderTestRecordName
		request.Name = testExpectedSyntheticTagName
		_, err := client.CreateReminderHashtag(ctx, *request)

		return err
	case reminderHashtagUpdateTestOperation:
		request := new(icloud.UpdateReminderHashtagRequest)
		request.Auth = auth
		request.Hashtag.ID = reminderHashtagTestName
		request.Name = testExpectedSyntheticTagName
		_, err := client.UpdateReminderHashtag(ctx, *request)

		return err
	case reminderHashtagDeleteTestOperation:
		request := new(icloud.DeleteReminderHashtagRequest)
		request.Auth = auth
		request.Reminder.ID = reminderTestRecordName
		request.Hashtag.ID = reminderHashtagTestName
		_, err := client.DeleteReminderHashtag(ctx, *request)

		return err
	default:
		panic("unknown hashtag operation")
	}
}
