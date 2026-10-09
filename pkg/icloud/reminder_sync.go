package icloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/portpowered/go-icloud/internal/webtransport"
)

const reminderSyncOperation = "GetReminderSyncCursor"

var errReminderSyncToken = errors.New("no usable reminder sync token")

type reminderSyncRead struct {
	sdk       *SDK
	auth      webtransport.RequestContext
	responses []*webtransport.BytesResponse
}

// GetReminderSyncCursor discovers a query token or the final fallback zone token.
// Every completed response is returned so the caller can persist cookie updates.
func (sdk *SDK) GetReminderSyncCursor(ctx context.Context,
	request GetReminderSyncCursorRequest,
) (*GetReminderSyncCursorResult, error) {
	auth, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(reminderSyncOperation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = request.Auth.RemindersServiceURL
	read := reminderSyncRead{sdk: sdk, auth: auth, responses: nil}

	token, err := read.cursor(ctx)
	if err != nil {
		return nil, read.failure(err)
	}

	responses := make([]ResponseMetadata, 0, len(read.responses))
	for _, response := range read.responses {
		responses = append(responses, publicMetadata(response))
	}

	return &GetReminderSyncCursorResult{SyncToken: token, Responses: responses}, nil
}

func (read *reminderSyncRead) cursor(ctx context.Context) (string, error) {
	query, err := read.sdk.web.ReminderCurrentSyncQuery(ctx, read.auth)
	if query != nil {
		read.responses = append(read.responses, query.Metadata)
	}

	if err != nil {
		var failure *webtransport.ResponseError
		if query == nil || !errors.As(err, &failure) || failure.Stage != webtransport.Decode {
			return "", fmt.Errorf("query reminder sync token: %w", err)
		}
	} else if token := query.Data.SyncToken.GetOrEmpty(); token != "" {
		return token, nil
	}

	return read.pages(ctx)
}

func (read *reminderSyncRead) pages(ctx context.Context) (string, error) {
	var syncToken *string

	moreComing := true
	for moreComing {
		response, err := read.sdk.web.ReminderSyncChanges(ctx, read.auth, syncToken)
		if err != nil {
			return "", fmt.Errorf("read reminder sync page: %w", err)
		}

		read.responses = append(read.responses, response.Metadata)
		moreComing = false

		if response.Data.Zones == nil || len(*response.Data.Zones) == 0 {
			break
		}

		for _, zone := range *response.Data.Zones {
			token := zone.SyncToken
			syncToken = &token
			moreComing = moreComing || zone.MoreComing.GetOrEmpty()
		}
	}

	if syncToken == nil || *syncToken == "" {
		return "", errReminderSyncToken
	}

	return *syncToken, nil
}

func (read *reminderSyncRead) failure(err error) *ClientError {
	var transportFailure *webtransport.ResponseError
	if errors.As(err, &transportFailure) {
		failure := adaptFailure(reminderSyncOperation, err)
		for _, response := range read.responses {
			failure.prior = append(failure.prior, publicMetadata(response))
		}

		return failure
	}

	last := read.responses[len(read.responses)-1]
	failure := newClientError(reminderSyncOperation, Provider, last.Status, last.Body,
		responseHeaders(last.Headers), err)
	failure.cookieScopeURL = last.CookieScopeURL

	for _, response := range read.responses[:len(read.responses)-1] {
		failure.prior = append(failure.prior, publicMetadata(response))
	}

	return failure
}
