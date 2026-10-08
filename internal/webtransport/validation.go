package webtransport

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/account"
)

func accountFields(body []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage

	err := json.Unmarshal(body, &fields)
	if err != nil {
		return nil, fmt.Errorf("decode account object: %w", err)
	}

	if fields == nil {
		return nil, errAccountShape
	}

	return fields, nil
}

func decodeFamily(body []byte) (account.AccountFamilyResponse, error) {
	var data account.AccountFamilyResponse

	fields, err := accountFields(body)
	if err != nil {
		return data, err
	}

	members, exists := fields[protocol.AccountFamilyResponseFamilyMembers]
	if exists && (len(members) == 0 || members[0] != '[') {
		return data, errAccountShape
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode family: %w", err)
	}

	return data, nil
}

func decodeStorage(body []byte) (account.AccountStorageResponse, error) {
	var data account.AccountStorageResponse

	fields, err := accountFields(body)
	if err != nil {
		return data, err
	}

	usage, err := accountFields(fields[protocol.AccountStorageResponseStorageUsageInfo])
	if err != nil {
		return data, err
	}

	for _, key := range []string{protocol.AccountStorageUsageUsedStorageInBytes,
		protocol.AccountStorageUsageTotalStorageInBytes} {
		if len(usage[key]) == 0 || string(usage[key]) == "null" {
			return data, errAccountShape
		}
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode storage: %w", err)
	}

	if data.StorageUsageInfo.UsedStorageInBytes < 0 || data.StorageUsageInfo.TotalStorageInBytes < 0 {
		return data, errAccountShape
	}

	err = validateMedia(fields[protocol.AccountStorageResponseStorageUsageByMedia])
	if err != nil {
		return data, err
	}

	return data, nil
}

func validateMedia(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var media []map[string]json.RawMessage

	err := json.Unmarshal(raw, &media)
	if err != nil {
		return fmt.Errorf("decode storage media: %w", err)
	}

	for _, entry := range media {
		if _, exists := entry[protocol.AccountMediaUsageMediaKey]; !exists {
			return errAccountShape
		}
	}

	return nil
}
