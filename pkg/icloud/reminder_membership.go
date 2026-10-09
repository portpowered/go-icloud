package icloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func (read *reminderListsRead) membership(ctx context.Context, record cloudkit.CKRecord) ([]string, error) {
	inline, err := reminderField(record, protocol.RemindersListFieldReminderIDsValue)
	if err != nil {
		return nil, err
	}

	if len(inline) != 0 && string(inline) != jsonNullValue {
		var text string

		err = json.Unmarshal(inline, &text)
		if err != nil {
			return nil, fmt.Errorf("decode inline reminder membership: %w", err)
		}

		return parseReminderMembership([]byte(text))
	}

	return read.assetMembership(ctx, record)
}

func (read *reminderListsRead) assetMembership(ctx context.Context, record cloudkit.CKRecord) ([]string, error) {
	raw, err := reminderField(record, protocol.RemindersListFieldReminderIDsAssetValue)
	if err != nil {
		return nil, err
	}

	if len(raw) == 0 || string(raw) == jsonNullValue {
		return []string{}, nil
	}

	var asset cloudkit.CKAssetToken

	err = json.Unmarshal(raw, &asset)
	if err != nil {
		return nil, fmt.Errorf("decode reminder membership token: %w", err)
	}

	if asset.DownloadedData.IsSpecified() && !asset.DownloadedData.IsNull() {
		content, decodeErr := base64.StdEncoding.DecodeString(asset.DownloadedData.GetOrEmpty())
		if decodeErr != nil {
			return nil, fmt.Errorf("decode reminder membership asset: %w", decodeErr)
		}

		return parseReminderMembership(content)
	}

	if asset.DownloadURL.GetOrEmpty() == "" {
		return nil, errReminderList
	}

	response, err := read.sdk.web.DownloadReminderMembership(ctx, read.auth, asset)
	if err != nil {
		return nil, fmt.Errorf("download reminder membership: %w", err)
	}

	read.responses = append(read.responses, response)

	return parseReminderMembership(response.Body)
}

func parseReminderMembership(content []byte) ([]string, error) {
	if !utf8.Valid(content) {
		return nil, errReminderList
	}

	var values []json.RawMessage

	err := json.Unmarshal(content, &values)
	if err != nil {
		return nil, fmt.Errorf("decode reminder membership array: %w", err)
	}

	if values == nil {
		return nil, errReminderList
	}

	ids := make([]string, 0, len(values))

	for _, value := range values {
		var identifier string

		if string(value) == jsonNullValue {
			return nil, errReminderList
		}

		err := json.Unmarshal(value, &identifier)
		if err != nil {
			return nil, fmt.Errorf("decode reminder member identifier: %w", err)
		}

		ids = append(ids, strings.TrimPrefix(identifier, protocol.RemindersReminderIDPrefixValue))
	}

	return ids, nil
}
