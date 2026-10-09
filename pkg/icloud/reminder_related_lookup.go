package icloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

type reminderRelatedRead struct {
	sdk       *SDK
	auth      webtransport.RequestContext
	operation string
	responses []*webtransport.BytesResponse
}

func (sdk *SDK) relatedRead(auth AuthContext, operation string) (*reminderRelatedRead, error) {
	boundary, err := accountRequestContext(auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	boundary.Origin = auth.RemindersServiceURL

	return &reminderRelatedRead{sdk: sdk, auth: boundary, operation: operation, responses: nil}, nil
}

func reminderRelatedNames(ids []string, prefix string) []string {
	names := make([]string, 0, len(ids))

	for _, id := range ids {
		if !strings.HasPrefix(id, prefix) {
			id = prefix + id
		}

		names = append(names, id)
	}

	return names
}

func (read *reminderRelatedRead) records(ctx context.Context,
	ids []string, prefix, kind string,
) ([]cloudkit.CKRecord, error) {
	records := []cloudkit.CKRecord{}
	if len(ids) == 0 {
		return records, nil
	}

	response, err := read.sdk.web.LookupReminders(ctx, read.auth, reminderRelatedNames(ids, prefix))
	if err != nil {
		return nil, fmt.Errorf("read related reminder records: %w", err)
	}

	read.responses = append(read.responses, response.Metadata)

	for _, item := range response.Data.Records {
		selected, decodeErr := webtransport.DecodeReminderLookupRecord(item)
		if decodeErr != nil {
			return nil, fmt.Errorf("decode related reminder record: %w", decodeErr)
		}

		if selected.Failure != nil {
			return nil, read.responseFailure(Provider, fmt.Errorf("%w: %s", errReminderList, selected.Failure.ServerErrorCode))
		}

		if selected.Record != nil && selected.Record.RecordType == kind {
			records = append(records, *selected.Record)
		}
	}

	return records, nil
}

func (read *reminderRelatedRead) metadata() []ResponseMetadata {
	responses := make([]ResponseMetadata, 0, len(read.responses))
	for _, response := range read.responses {
		responses = append(responses, publicMetadata(response))
	}

	return responses
}

func (read *reminderRelatedRead) failure(err error) *ClientError {
	var clientFailure *ClientError
	if errors.As(err, &clientFailure) {
		return clientFailure
	}

	var transportFailure *webtransport.ResponseError
	if errors.As(err, &transportFailure) {
		failure := adaptFailure(read.operation, err)
		failure.prior = read.metadata()

		return failure
	}

	return read.responseFailure(InvalidResponse, err)
}

func (read *reminderRelatedRead) responseFailure(kind ErrorKind, err error) *ClientError {
	if len(read.responses) == 0 {
		return newClientError(read.operation, kind, 0, nil, nil, err)
	}

	last := read.responses[len(read.responses)-1]
	failure := newClientError(read.operation, kind, last.Status, last.Body, responseHeaders(last.Headers), err)

	failure.cookieScopeURL = last.CookieScopeURL

	for _, response := range read.responses[:len(read.responses)-1] {
		failure.prior = append(failure.prior, publicMetadata(response))
	}

	return failure
}
