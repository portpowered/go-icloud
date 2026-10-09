package webtransport

import "encoding/json"

func reminderSyncArray(raw json.RawMessage, validate func(json.RawMessage) bool) bool {
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil || items == nil {
		return false
	}

	for _, item := range items {
		if string(item) == jsonNullValue || !validate(item) {
			return false
		}
	}

	return true
}

func reminderSyncStrings(raw json.RawMessage) bool {
	return reminderSyncArray(raw, reminderSyncString)
}

func reminderSyncReferences(raw json.RawMessage) bool {
	return reminderSyncArray(raw, reminderSyncReference)
}

func reminderSyncAssets(raw json.RawMessage) bool {
	return reminderSyncArray(raw, reminderSyncAsset)
}

func reminderSyncDoubles(raw json.RawMessage) bool {
	return reminderSyncArray(raw, reminderSyncDouble)
}

func reminderSyncIntegers(raw json.RawMessage) bool {
	return reminderSyncArray(raw, reminderSyncInteger)
}

func reminderSyncUnknownList(raw json.RawMessage) bool {
	var items []json.RawMessage

	return json.Unmarshal(raw, &items) == nil && items != nil
}
