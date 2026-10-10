package icloud

import (
	"context"
	"errors"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
)

const reminderChangesOperation = "ListReminderChanges"

type reminderChangesRead struct {
	sdk       *SDK
	auth      webtransport.RequestContext
	responses []*webtransport.BytesResponse
	changes   []ReminderChangeEvent
}

// ListReminderChanges consumes all pages, preserving event order and duplicates.
// A failed read returns response evidence through ClientError, without a partial result.
func (sdk *SDK) ListReminderChanges(ctx context.Context,
	request ListReminderChangesRequest,
) (*ListReminderChangesResult, error) {
	auth, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(reminderChangesOperation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = request.Auth.RemindersServiceURL
	read := reminderChangesRead{sdk: sdk, auth: auth, responses: nil, changes: make([]ReminderChangeEvent, 0)}

	err = read.pages(ctx, request.Since)
	if err != nil {
		return nil, read.failure(err)
	}

	responses := make([]ResponseMetadata, 0, len(read.responses))
	for _, response := range read.responses {
		responses = append(responses, publicMetadata(response))
	}

	return &ListReminderChangesResult{Changes: read.changes, Responses: responses}, nil
}

func (read *reminderChangesRead) pages(ctx context.Context, since *string) error {
	moreComing := true
	for moreComing {
		response, err := read.sdk.web.ReminderEventChanges(ctx, read.auth, since)
		if err != nil {
			return fmt.Errorf("read reminder change page: %w", err)
		}

		read.responses = append(read.responses, response.Metadata)
		moreComing = false

		if response.Data.Zones == nil {
			return nil
		}

		for _, zone := range *response.Data.Zones {
			if zone.Records != nil {
				for _, item := range *zone.Records {
					err = read.event(item)
					if err != nil {
						return err
					}
				}
			}

			token := zone.SyncToken
			since = &token
			moreComing = moreComing || zone.MoreComing.GetOrEmpty()
		}
	}

	return nil
}

func (read *reminderChangesRead) failure(err error) *ClientError {
	var transportFailure *webtransport.ResponseError
	if errors.As(err, &transportFailure) {
		failure := adaptFailure(reminderChangesOperation, err)
		for _, response := range read.responses {
			failure.prior = append(failure.prior, publicMetadata(response))
		}

		return failure
	}

	var clientFailure *ClientError
	if errors.As(err, &clientFailure) {
		return clientFailure
	}

	return read.responseFailure(InvalidResponse, err)
}

func (read *reminderChangesRead) responseFailure(kind ErrorKind, err error) *ClientError {
	last := read.responses[len(read.responses)-1]
	failure := newClientError(reminderChangesOperation, kind, last.Status, last.Body, responseHeaders(last.Headers), err)
	failure.cookieScopeURL = last.CookieScopeURL

	for _, response := range read.responses[:len(read.responses)-1] {
		failure.prior = append(failure.prior, publicMetadata(response))
	}

	return failure
}
