package icloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const (
	reminderListsOperation = "ListReminderLists"
	jsonNullValue          = "null"
)

var errReminderList = errors.New("invalid reminder list record or membership")

type reminderListsRead struct {
	sdk       *SDK
	auth      webtransport.RequestContext
	responses []*webtransport.BytesResponse
	lists     []ReminderList
}

// ListReminderLists consumes every list page and provider-issued membership asset.
// Authentication and cookies remain local to this operation.
func (sdk *SDK) ListReminderLists(ctx context.Context,
	request ListReminderListsRequest,
) (*ListReminderListsResult, error) {
	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(reminderListsOperation, Configuration, 0, nil, nil, err)
	}

	boundary.Origin = request.Auth.RemindersServiceURL

	read := reminderListsRead{sdk: sdk, auth: boundary, responses: nil, lists: make([]ReminderList, 0)}

	err = read.pages(ctx)
	if err != nil {
		return nil, read.failure(err)
	}

	responses := make([]ResponseMetadata, 0, len(read.responses))
	for _, response := range read.responses {
		responses = append(responses, publicMetadata(response))
	}

	return &ListReminderListsResult{Lists: read.lists, Responses: responses}, nil
}

func (read *reminderListsRead) pages(ctx context.Context) error {
	var syncToken *string

	moreComing := true
	for moreComing {
		response, err := read.sdk.web.ReminderListChanges(ctx, read.auth, syncToken)
		if err != nil {
			return fmt.Errorf("read reminder list page: %w", err)
		}

		read.responses = append(read.responses, response.Metadata)
		moreComing = false

		if response.Data.Zones == nil {
			return nil
		}

		for _, zone := range *response.Data.Zones {
			err = read.zone(ctx, zone)
			if err != nil {
				return err
			}

			token := zone.SyncToken
			syncToken = &token
			moreComing = moreComing || zone.MoreComing.GetOrEmpty()
		}
	}

	return nil
}

func (read *reminderListsRead) zone(ctx context.Context, zone cloudkit.CKZoneChangesZone) error {
	if zone.Records == nil {
		return nil
	}

	records := make([]cloudkit.CKRecord, 0, len(*zone.Records))
	// Source rejects any error in the zone before projecting its first record.
	for _, item := range *zone.Records {
		selected, err := webtransport.DecodeReminderEventRecord(item)
		if err != nil {
			return fmt.Errorf("decode reminder list record: %w", err)
		}

		if selected.Failure != nil {
			return read.providerFailure(fmt.Errorf("%w: %s", errReminderList, selected.Failure.ServerErrorCode))
		}

		if selected.Record != nil {
			records = append(records, *selected.Record)
		}
	}

	for _, record := range records {
		if record.RecordType != protocol.RemindersListRecordTypeValue {
			continue
		}

		list, err := read.list(ctx, record)
		if err != nil {
			return err
		}

		read.lists = append(read.lists, list)
	}

	return nil
}

func (read *reminderListsRead) failure(err error) *ClientError {
	var transportFailure *webtransport.ResponseError
	if errors.As(err, &transportFailure) {
		failure := adaptFailure(reminderListsOperation, err)
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

func (read *reminderListsRead) providerFailure(err error) *ClientError {
	return read.responseFailure(Provider, err)
}

func (read *reminderListsRead) responseFailure(kind ErrorKind, err error) *ClientError {
	if len(read.responses) == 0 {
		return newClientError(reminderListsOperation, kind, 0, nil, nil, err)
	}

	last := read.responses[len(read.responses)-1]
	failure := newClientError(reminderListsOperation, kind, last.Status, last.Body, responseHeaders(last.Headers), err)

	failure.cookieScopeURL = last.CookieScopeURL

	for _, response := range read.responses[:len(read.responses)-1] {
		failure.prior = append(failure.prior, publicMetadata(response))
	}

	return failure
}

func reminderField(record cloudkit.CKRecord, name string) (json.RawMessage, error) {
	if record.Fields == nil {
		return nil, nil
	}

	field, exists := (*record.Fields)[name]
	if !exists {
		return nil, nil
	}

	value, err := field.AsCKPassthroughField()
	if err != nil {
		return nil, fmt.Errorf("decode reminder field: %w", err)
	}

	if value.Type == string(cloudkit.CKInt64FieldTypeINT64) {
		integer, integerErr := reminderWireInteger(value.Value)
		if integerErr != nil {
			return nil, integerErr
		}

		return json.RawMessage(strconv.FormatInt(integer, 10)), nil
	}

	return value.Value, nil
}
