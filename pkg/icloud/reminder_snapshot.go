package icloud

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
)

const reminderSnapshotOperation = "ListReminderSnapshot"

type reminderSnapshotRead struct {
	sdk       *SDK
	auth      webtransport.RequestContext
	responses []*webtransport.BytesResponse
	reminders []Reminder
	positions map[string]int
}

// ListReminderSnapshot collects reminders including completed items across selected lists.
// Omitted or empty list filters discover lists before querying them; all pages are consumed.
func (sdk *SDK) ListReminderSnapshot(ctx context.Context,
	request ListReminderSnapshotRequest,
) (*ListReminderSnapshotResult, error) {
	auth, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(reminderSnapshotOperation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = request.Auth.RemindersServiceURL
	read := reminderSnapshotRead{sdk: sdk, auth: auth, responses: nil,
		reminders: []Reminder{}, positions: map[string]int{}}

	lists, err := read.lists(ctx, request.ListID)
	if err != nil {
		return nil, err
	}

	for _, list := range lists {
		err = read.query(ctx, list)
		if err != nil {
			return nil, err
		}
	}

	result := &ListReminderSnapshotResult{Reminders: read.reminders, Responses: []ResponseMetadata{}}
	for _, response := range read.responses {
		result.Responses = append(result.Responses, publicMetadata(response))
	}

	return result, nil
}

func (read *reminderSnapshotRead) lists(ctx context.Context, filter *string) ([]string, error) {
	if filter != nil && *filter != "" {
		return []string{*filter}, nil
	}

	discovery := reminderListsRead{sdk: read.sdk, auth: read.auth, responses: nil, lists: []ReminderList{}}

	err := discovery.pages(ctx)
	read.responses = discovery.responses

	if err != nil {
		return nil, reminderSnapshotFailure(discovery.failure(err))
	}

	ids := make([]string, 0, len(discovery.lists))
	for _, list := range discovery.lists {
		ids = append(ids, list.ID)
	}

	return ids, nil
}

func (read *reminderSnapshotRead) query(ctx context.Context, list string) error {
	query := reminderQueryRead{sdk: read.sdk, auth: read.auth, result: newReminderQueryResult(),
		positions: map[string]int{}, responses: read.responses}
	completed := true
	request := new(ListRemindersRequest)
	request.ListID, request.IncludeCompleted = list, &completed

	err := query.pages(ctx, *request)
	read.responses = query.responses

	if err != nil {
		return reminderSnapshotFailure(query.failure(err))
	}

	err = scopeReminderQuery(query.result, list)
	if err != nil {
		return reminderSnapshotFailure(query.failure(err))
	}

	for _, reminder := range query.result.Reminders {
		if position, exists := read.positions[reminder.ID]; exists {
			read.reminders[position] = reminder
		} else {
			read.positions[reminder.ID] = len(read.reminders)
			read.reminders = append(read.reminders, reminder)
		}
	}

	return nil
}

func reminderSnapshotFailure(cause *ClientError) *ClientError {
	failure := *cause
	failure.operation = reminderSnapshotOperation

	return &failure
}
