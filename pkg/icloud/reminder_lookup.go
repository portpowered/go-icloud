package icloud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const getReminderOperation = "GetReminder"

var errReminderMissing = errors.New("reminder not found")

// GetReminder reads one complete reminder, preserving response evidence and cookie updates.
func (sdk *SDK) GetReminder(ctx context.Context, request GetReminderRequest) (*GetReminderResult, error) {
	auth, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(getReminderOperation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = request.Auth.RemindersServiceURL

	name := request.ReminderID
	if name != "" && !strings.HasPrefix(name, protocol.RemindersReminderIDPrefixValue) {
		name = protocol.RemindersReminderIDPrefixValue + name
	}

	response, err := sdk.web.LookupReminder(ctx, auth, name)
	if err != nil {
		return nil, adaptFailure(getReminderOperation, err)
	}

	record, kind, err := selectReminderRecord(response.Data, name)
	if err != nil {
		return nil, reminderLookupFailure(kind, err, response.Metadata)
	}

	reminder, err := projectReminder(record)
	if err != nil {
		return nil, reminderLookupFailure(InvalidResponse, err, response.Metadata)
	}

	return &GetReminderResult{Reminder: reminder, Metadata: publicMetadata(response.Metadata)}, nil
}

func selectReminderRecord(data cloudkit.CKLookupResponse, name string) (cloudkit.CKRecord, ErrorKind, error) {
	for _, item := range data.Records {
		recordError, err := item.AsCKErrorItem()
		if err != nil {
			return cloudkit.CKRecord{}, InvalidResponse, fmt.Errorf("decode reminder error: %w", err)
		}

		if recordError.ServerErrorCode != "" {
			return cloudkit.CKRecord{}, Provider, fmt.Errorf("%w: %s", errReminderList, recordError.ServerErrorCode)
		}
	}

	for _, item := range data.Records {
		record, err := item.AsCKRecord()
		if err != nil {
			return record, InvalidResponse, fmt.Errorf("decode reminder record: %w", err)
		}

		if record.RecordType != "" && record.RecordName == name {
			return record, "", nil
		}
	}

	return cloudkit.CKRecord{}, NotFound, errReminderMissing
}

func reminderLookupFailure(kind ErrorKind, err error, response *webtransport.BytesResponse) *ClientError {
	result := newClientError(getReminderOperation, kind, response.Status, response.Body,
		responseHeaders(response.Headers), err)
	result.cookieScopeURL = response.CookieScopeURL

	return result
}
