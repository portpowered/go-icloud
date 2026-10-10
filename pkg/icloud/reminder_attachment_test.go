package icloud_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const (
	reminderAttachmentCancelCreate = "cancel-create"
	reminderAttachmentCancelDelete = "cancel-delete"
	reminderAttachmentWrongParent  = "wrong-parent"
	reminderAttachmentServiceURL   = "https://reminders.example.invalid"
)

func TestReminderAttachmentValidationBeforeNetwork(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{testExpectedURLEmptyUpdate, "image-empty-update", testInvalidUnion,
		testExpectedNegativeSize, testExpectedNegativeWidth, testNegativeHeight, reminderAttachmentWrongParent,
		reminderAttachmentCancelCreate, testCancelUpdate, reminderAttachmentCancelDelete} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			checkAttachmentPreparation(t, mode)
		})
	}
}

func checkAttachmentPreparation(t *testing.T, mode string) {
	t.Helper()

	var entropy reminderFailingEntropy

	client, err := icloud.New(icloud.WithRandomSource(&entropy),
		icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
			t.Fatal("invalid attachment reached the transport")

			return nil, errSyntheticPeer
		})))
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	kind := icloud.Configuration

	if mode == reminderAttachmentCancelCreate || mode == testCancelUpdate || mode == reminderAttachmentCancelDelete {
		cancel()

		kind = icloud.Canceled
	}

	err = callAttachmentPreparation(ctx, client, mode)

	var failure *icloud.ClientError

	if !errors.As(err, &failure) || failure.Kind() != kind || entropy.calls != 0 {
		t.Fatal("attachment preparation lost error class or consumed entropy", err, entropy.calls)
	}

	if kind == icloud.Canceled && !errors.Is(err, context.Canceled) {
		t.Fatal("attachment preparation lost context cause", err)
	}
}

//nolint:wrapcheck // LIB-05: preserve the client error for exact classification assertions.
func callAttachmentPreparation(ctx context.Context, client *icloud.SDK, mode string) error {
	auth := deviceRequest().Auth
	auth.RemindersServiceURL = reminderAttachmentServiceURL

	attachment := preparationAttachment(mode)

	if mode == reminderAttachmentCancelCreate {
		request := new(icloud.CreateReminderURLAttachmentRequest)
		request.Auth = auth
		_, err := client.CreateReminderURLAttachment(ctx, *request)

		return err
	}

	if mode == reminderAttachmentWrongParent || mode == reminderAttachmentCancelDelete {
		request := new(icloud.DeleteReminderAttachmentRequest)
		request.Auth, request.Attachment = auth, attachment
		request.Reminder.ID = reminderTestRecordName
		_, err := client.DeleteReminderAttachment(ctx, *request)

		return err
	}

	request := new(icloud.UpdateReminderAttachmentRequest)
	request.Auth, request.Attachment = auth, attachment
	invalid := int64(-1)
	assignAttachmentInvalidValue(request, mode, &invalid)

	before, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode initial attachment request: %w", err)
	}

	_, err = client.UpdateReminderAttachment(ctx, *request)

	if !attachmentRequestUnchanged(request, before) {
		return errSyntheticPeer
	}

	return err
}

func preparationAttachment(mode string) icloud.ReminderAttachment {
	attachment := new(icloud.ReminderAttachment)
	if mode == testInvalidUnion {
		return *attachment
	}

	if mode == testExpectedURLEmptyUpdate || mode == reminderAttachmentWrongParent {
		url := new(icloud.ReminderURLAttachment)
		url.ID = testAttachmentID
		url.ReminderID = "Reminder/another"
		_ = attachment.FromReminderURLAttachment(*url)
	} else {
		image := new(icloud.ReminderImageAttachment)
		image.ID = testAttachmentID
		_ = attachment.FromReminderImageAttachment(*image)
	}

	return *attachment
}

func attachmentRequestUnchanged(request *icloud.UpdateReminderAttachmentRequest, before []byte) bool {
	after, err := json.Marshal(request)

	return err == nil && string(before) == string(after)
}

func assignAttachmentInvalidValue(request *icloud.UpdateReminderAttachmentRequest, mode string, value *int64) {
	switch mode {
	case testExpectedNegativeSize:
		request.FileSize = value
	case testExpectedNegativeWidth:
		request.Width = value
	case testNegativeHeight:
		request.Height = value
	}
}
