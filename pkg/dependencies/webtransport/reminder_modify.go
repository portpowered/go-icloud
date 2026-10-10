package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/remindersapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ReminderModificationResponse owns the generated modify response and exact provider evidence.
type ReminderModificationResponse struct {
	Data     cloudkit.CKModifyResponse
	Metadata *BytesResponse
}

// CreateReminderHashtag sends the atomic parent-link and child creation.
func (client *Client) CreateReminderHashtag(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderHashtagCreationRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}

// UpdateReminderHashtag sends a revision-aware hashtag rename.
func (client *Client) UpdateReminderHashtag(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderHashtagUpdateRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}

// DeleteReminderHashtag sends the atomic parent unlink and child soft deletion.
func (client *Client) DeleteReminderHashtag(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderHashtagDeletionRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}

// DeleteReminder sends a schema-owned revision-aware soft-delete operation.
func (client *Client) DeleteReminder(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderDeletionRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}

// CreateReminder sends a schema-owned reminder creation.
func (client *Client) CreateReminder(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderCreationRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}

// UpdateReminder sends a schema-owned complete reminder update.
func (client *Client) UpdateReminder(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderUpdateRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}

func (client *Client) modifyReminder(ctx context.Context, auth RequestContext,
	body []byte, err error,
) (*ReminderModificationResponse, error) {
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := new(remindersapi.RemindersModifyRecordsParams)
	params.ClientId = auth.Params.ClientId
	params.Dsid = auth.Params.Dsid
	params.RemapEnums = remindersapi.RemindersModifyRecordsParamsRemapEnumsTrue
	params.GetCurrentSyncToken = remindersapi.RemindersModifyRecordsParamsGetCurrentSyncTokenTrue

	request, err := remindersapi.NewRemindersModifyRecordsRequestWithBody(auth.Origin, params,
		protocol.RemindersMediaApplicationJson, bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.readReminderRequest(ctx, auth, request)
	if err != nil {
		return nil, err
	}

	data, err := decodeReminderModification(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, fmt.Errorf("decode reminder modification: %w", err), response)
	}

	return &ReminderModificationResponse{Data: data, Metadata: response}, nil
}

func decodeReminderModification(body []byte) (cloudkit.CKModifyResponse, error) {
	var data cloudkit.CKModifyResponse

	fields, err := accountFields(body)
	if err != nil {
		return data, err
	}

	if raw := fields[protocol.RemindersCKModifyResponseRecords]; len(raw) != 0 {
		err = validateReminderChangeRecords(raw)
		if err != nil {
			return data, err
		}
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode reminder modification envelope: %w", err)
	}

	err = validateReminderSyncRecords(data.Records)

	return data, err
}
