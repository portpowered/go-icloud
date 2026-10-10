package webtransport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/remindersapi"
)

var errLegacyReminderShape = errors.New("legacy reminder startup must contain list and reminder object arrays")

// LegacyRemindersResponse retains the startup records and complete response evidence.
type LegacyRemindersResponse struct {
	Data     remindersapi.LegacyRemindersStartup
	Metadata *BytesResponse
}

// LegacyRemindersStartup reads the discovered legacy service without CloudKit initialization.
func (client *Client) LegacyRemindersStartup(ctx context.Context,
	auth RequestContext,
) (*LegacyRemindersResponse, error) {
	err := validateOrigin(auth.Origin)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	params := remindersapi.RemindersLegacyStartupParams{ClientId: auth.Params.ClientId, Dsid: auth.Params.Dsid,
		ClientBuildNumber: auth.Params.ClientBuildNumber, ClientMasteringNumber: auth.Params.ClientMasteringNumber}

	request, err := remindersapi.NewRemindersLegacyStartupRequest(auth.Origin, &params)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request = request.WithContext(ctx)
	request.Header = CallerHeaders(auth.Headers)
	request.URL.RawQuery = orderedAccountQuery(auth.Params)

	response, err := client.readPrepared(request, successfulContent, auth.Cookies)
	if err != nil {
		return nil, err
	}

	var data remindersapi.LegacyRemindersStartup

	err = json.Unmarshal(response.Body, &data)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	if data.Collections == nil || data.Reminders == nil ||
		!legacyReminderObjects(data.Collections) || !legacyReminderObjects(data.Reminders) {
		return nil, responseFailure(Decode, errLegacyReminderShape, response)
	}

	return &LegacyRemindersResponse{Data: data, Metadata: response}, nil
}

func legacyReminderObjects(records []json.RawMessage) bool {
	for _, record := range records {
		value := bytes.TrimSpace(record)
		if len(value) == 0 || value[0] != '{' {
			return false
		}
	}

	return true
}
