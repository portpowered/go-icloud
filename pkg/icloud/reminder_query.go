package icloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const listRemindersOperation = "ListReminders"

type reminderQueryRead struct {
	sdk       *SDK
	auth      webtransport.RequestContext
	result    *ListRemindersResult
	positions map[string]int
	responses []*webtransport.BytesResponse
}

// ListReminders consumes every compound query page and scopes related records to the requested list.
func (sdk *SDK) ListReminders(ctx context.Context, request ListRemindersRequest) (*ListRemindersResult, error) {
	auth, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(listRemindersOperation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = request.Auth.RemindersServiceURL
	result := newReminderQueryResult()
	read := reminderQueryRead{sdk: sdk, auth: auth, result: result, positions: map[string]int{}, responses: nil}

	err = read.pages(ctx, request)
	if err != nil {
		return nil, read.failure(err)
	}

	err = scopeReminderQuery(result, request.ListID)
	if err != nil {
		return nil, read.failure(err)
	}

	for _, response := range read.responses {
		result.Responses = append(result.Responses, publicMetadata(response))
	}

	return result, nil
}

func newReminderQueryResult() *ListRemindersResult {
	return &ListRemindersResult{Reminders: []Reminder{}, Alarms: map[string]ReminderAlarm{},
		Triggers: map[string]ReminderLocationTrigger{}, Attachments: map[string]ReminderAttachment{},
		Hashtags: map[string]ReminderHashtag{}, RecurrenceRules: map[string]ReminderRecurrenceRule{},
		Responses: []ResponseMetadata{}}
}

func (read *reminderQueryRead) pages(ctx context.Context, request ListRemindersRequest) error {
	limit := int64(cloudkit.N200)
	if request.ResultsLimit != nil {
		limit = *request.ResultsLimit
	}

	completed := request.IncludeCompleted != nil && *request.IncludeCompleted

	var continuation *string

	for {
		response, err := read.sdk.web.ReminderCompoundQuery(ctx, read.auth, request.ListID, completed, limit, continuation)
		if err != nil {
			return fmt.Errorf("read compound reminder page: %w", err)
		}

		read.responses = append(read.responses, response.Metadata)

		err = read.page(response.Data)
		if err != nil {
			return err
		}

		marker := response.Data.ContinuationMarker.GetOrEmpty()
		if marker == "" {
			return nil
		}

		continuation = &marker
	}
}

func (read *reminderQueryRead) page(data cloudkit.CKQueryResponse) error {
	if data.Records == nil {
		return nil
	}

	records := make([]cloudkit.CKRecord, 0, len(*data.Records))

	for _, item := range *data.Records {
		value, err := webtransport.DecodeReminderQueryRecord(item)
		if err != nil {
			return fmt.Errorf("decode compound reminder record: %w", err)
		}

		if value.Failure != nil {
			return read.responseFailure(Provider, fmt.Errorf("%w: %s", errReminderList, value.Failure.ServerErrorCode))
		}

		if value.Record != nil {
			records = append(records, *value.Record)
		}
	}

	for _, record := range records {
		err := read.ingest(record)
		if err != nil {
			return err
		}
	}

	return nil
}

func (read *reminderQueryRead) failure(err error) *ClientError {
	var clientFailure *ClientError
	if errors.As(err, &clientFailure) {
		return clientFailure
	}

	var transportFailure *webtransport.ResponseError
	if errors.As(err, &transportFailure) {
		failure := adaptFailure(listRemindersOperation, err)
		for _, response := range read.responses {
			failure.prior = append(failure.prior, publicMetadata(response))
		}

		return failure
	}

	return read.responseFailure(InvalidResponse, err)
}

func (read *reminderQueryRead) responseFailure(kind ErrorKind, err error) *ClientError {
	if len(read.responses) == 0 {
		return newClientError(listRemindersOperation, kind, 0, nil, nil, err)
	}

	last := read.responses[len(read.responses)-1]
	failure := newClientError(listRemindersOperation, kind, last.Status, last.Body, responseHeaders(last.Headers), err)
	failure.cookieScopeURL = last.CookieScopeURL

	for _, response := range read.responses[:len(read.responses)-1] {
		failure.prior = append(failure.prior, publicMetadata(response))
	}

	return failure
}
