package icloud_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"slices"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const (
	recurrenceKeepID         = "keep"
	recurrenceInvalidJSON    = `not JSON`
	recurrenceInvalidRecords = `{"records":[false]}`
	recurrenceNullRecords    = `{"records":null}`
	recurrenceTestCreate     = "create"
	recurrenceTestUpdate     = "update"
	recurrenceTestDelete     = "delete"
	recurrenceServiceOrigin  = "https://reminders.example.invalid"
)

func recurrenceTestAuth() icloud.AuthContext {
	auth := deviceRequest().Auth
	auth.RemindersServiceURL = recurrenceServiceOrigin

	return auth
}

func TestReminderRecurrenceUnlinkRemovesEmptyIDs(t *testing.T) {
	t.Parallel()
	client := sdkForResponse(t, http.StatusOK, `{}`)

	var reminder icloud.Reminder

	reminder.ID = reminderTestRecordName
	reminder.RecurrenceRuleIDs = []string{"", testRetainedRecurrence, recurrenceKeepID, ""}
	rule := recurrenceTestRule()
	rule.ID = ""

	result, err := client.DeleteReminderRecurrenceRule(t.Context(), icloud.DeleteReminderRecurrenceRuleRequest{
		Auth: recurrenceTestAuth(), Reminder: reminder, RecurrenceRule: rule})
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(result.Reminder.RecurrenceRuleIDs, []string{recurrenceKeepID, recurrenceKeepID}) {
		t.Fatal("empty child unlink lost ordered duplicate IDs", result.Reminder.RecurrenceRuleIDs)
	}

	if !slices.Equal(reminder.RecurrenceRuleIDs, []string{"", testRetainedRecurrence, recurrenceKeepID, ""}) {
		t.Fatal("unlink mutated caller ID list")
	}
}

func recurrenceTestRule() icloud.ReminderRecurrenceRule {
	return icloud.ReminderRecurrenceRule{ID: "RecurrenceRule/synthetic", ReminderID: reminderTestRecordName,
		Frequency: icloud.ReminderDaily, Interval: 1, OccurrenceCount: 0, FirstDayOfWeek: 0, RecordChangeTag: nil}
}

func TestReminderRecurrencePreparationGuards(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{recurrenceTestCreate, recurrenceTestUpdate, recurrenceTestDelete} {
		for _, canceled := range []bool{true, false} {
			if operation == recurrenceTestUpdate && !canceled {
				continue
			}

			t.Run(operation+map[bool]string{true: testCanceledSuffix, false: testExpectedEntropy}[canceled], func(t *testing.T) {
				t.Parallel()

				entropy := new(reminderFailingEntropy)
				client := recurrenceGuardClient(t, entropy)

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				kind, cause, reads := icloud.Configuration, io.ErrUnexpectedEOF, 1

				if canceled {
					cancel()

					kind, cause, reads = icloud.Canceled, context.Canceled, 0
				}

				err := callRecurrenceMutation(ctx, client, operation)

				var failure *icloud.ClientError

				if !errors.As(err, &failure) || failure.Kind() != kind || !errors.Is(err, cause) || entropy.calls != reads {
					t.Fatal("incorrect recurrence preparation behavior", err, entropy.calls)
				}
			})
		}
	}
}

func recurrenceGuardClient(t *testing.T, entropy io.Reader) *icloud.SDK {
	t.Helper()

	client, err := icloud.New(icloud.WithRandomSource(entropy), icloud.WithHTTPTransport(sdkRoundTrip(
		func(_ *http.Request) (*http.Response, error) {
			t.Fatal("invalid recurrence preparation sent a request")

			return nil, errSyntheticPeer
		})))
	if err != nil {
		t.Fatal(err)
	}

	return client
}

//nolint:wrapcheck // LIB-05: preserve the public SDK error for failure evidence assertions.
func callRecurrenceMutation(ctx context.Context, client *icloud.SDK, operation string) error {
	var reminder icloud.Reminder

	reminder.ID = reminderTestRecordName

	switch operation {
	case recurrenceTestCreate:
		request := new(icloud.CreateReminderRecurrenceRuleRequest)
		request.Auth = recurrenceTestAuth()
		request.Reminder = reminder
		_, err := client.CreateReminderRecurrenceRule(ctx, *request)

		return err
	case recurrenceTestUpdate:
		interval := int64(2)
		request := new(icloud.UpdateReminderRecurrenceRuleRequest)
		request.Auth = recurrenceTestAuth()
		request.RecurrenceRule = recurrenceTestRule()
		request.Interval = &interval
		_, err := client.UpdateReminderRecurrenceRule(ctx, *request)

		return err
	default:
		_, err := client.DeleteReminderRecurrenceRule(ctx, icloud.DeleteReminderRecurrenceRuleRequest{
			Auth: recurrenceTestAuth(), Reminder: reminder, RecurrenceRule: recurrenceTestRule()})

		return err
	}
}

func TestReminderRecurrenceValidationHasNoEffects(t *testing.T) {
	t.Parallel()

	for name, rule := range map[string]icloud.ReminderRecurrenceRule{
		"frequency": {ID: "", ReminderID: "", Frequency: 0, Interval: 1,
			OccurrenceCount: 0, FirstDayOfWeek: 0, RecordChangeTag: nil},
		"interval": {ID: "", ReminderID: "", Frequency: icloud.ReminderDaily, Interval: 0,
			OccurrenceCount: 0, FirstDayOfWeek: 0, RecordChangeTag: nil},
		"count": {ID: "", ReminderID: "", Frequency: icloud.ReminderDaily, Interval: 1,
			OccurrenceCount: -1, FirstDayOfWeek: 0, RecordChangeTag: nil},
		"weekday": {ID: "", ReminderID: "", Frequency: icloud.ReminderDaily, Interval: 1,
			OccurrenceCount: 0, FirstDayOfWeek: 7, RecordChangeTag: nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			entropy := new(reminderFailingEntropy)
			client := recurrenceGuardClient(t, entropy)
			request := new(icloud.CreateReminderRecurrenceRuleRequest)
			request.Auth = recurrenceTestAuth()
			request.Frequency = &rule.Frequency
			request.Interval = &rule.Interval
			request.OccurrenceCount = &rule.OccurrenceCount
			request.FirstDayOfWeek = &rule.FirstDayOfWeek
			result, err := client.CreateReminderRecurrenceRule(t.Context(), *request)
			checkRecurrenceValidationError(t, err, result == nil, entropy.calls)
		})
	}
}

func checkRecurrenceValidationError(t *testing.T, err error, noResult bool, reads int) {
	t.Helper()

	var failure *icloud.ClientError
	if !noResult || !errors.As(err, &failure) || failure.Kind() != icloud.Configuration || reads != 0 {
		t.Fatal("invalid recurrence settings had side effects", err, reads)
	}
}

func TestReminderRecurrenceEmptyUpdateAndWrongParentHaveNoEffects(t *testing.T) {
	t.Parallel()

	entropy := new(reminderFailingEntropy)
	client := recurrenceGuardClient(t, entropy)
	request := new(icloud.UpdateReminderRecurrenceRuleRequest)
	request.Auth = recurrenceTestAuth()
	request.RecurrenceRule = recurrenceTestRule()
	result, err := client.UpdateReminderRecurrenceRule(t.Context(), *request)
	checkRecurrenceValidationError(t, err, result == nil, entropy.calls)

	var reminder icloud.Reminder

	reminder.ID = "Reminder/other"
	removed, err := client.DeleteReminderRecurrenceRule(t.Context(), icloud.DeleteReminderRecurrenceRuleRequest{
		Auth: recurrenceTestAuth(), Reminder: reminder, RecurrenceRule: recurrenceTestRule()})
	checkRecurrenceValidationError(t, err, removed == nil, entropy.calls)
}

func TestReminderRecurrenceMalformedResponsesPreserveEvidence(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{recurrenceTestCreate, recurrenceTestUpdate, recurrenceTestDelete} {
		for _, body := range []string{recurrenceNullRecords, recurrenceInvalidRecords, recurrenceInvalidJSON} {
			t.Run(operation+body, func(t *testing.T) {
				t.Parallel()
				err := callRecurrenceMutation(t.Context(), sdkForResponse(t, http.StatusOK, body), operation)

				var failure *icloud.ClientError

				if !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse || string(failure.ResponseBody()) != body {
					t.Fatal("recurrence failure lost response evidence", err)
				}

				if len(failure.ResponseHeaders()) == 0 || failure.CookieScopeURL() == "" {
					t.Fatal("recurrence failure lost metadata")
				}
			})
		}
	}
}
