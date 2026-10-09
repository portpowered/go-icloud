package icloud

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func (read *reminderListsRead) list(ctx context.Context, record cloudkit.CKRecord) (ReminderList, error) {
	result := ReminderList{ID: record.RecordName, Title: protocol.RemindersUntitledListValue,
		Color: nil, Count: 0, BadgeEmblem: nil, SortingStyle: nil, IsGroup: false, Deleted: false,
		ReminderIDs: nil, Guid: nil, RecordChangeTag: record.RecordChangeTag}
	result.Guid.SetNull()

	if !result.RecordChangeTag.IsSpecified() {
		result.RecordChangeTag.SetNull()
	}

	if result.ID == "" {
		return result, errReminderList
	}

	ids, err := read.membership(ctx, record)
	if err != nil {
		return result, err
	}

	result.ReminderIDs = ids

	err = projectReminderListFields(&result, record)
	if err != nil {
		return result, err
	}

	if result.Count == 0 {
		result.Count = int64(len(ids))
	}

	return result, nil
}

func projectReminderListFields(result *ReminderList, record cloudkit.CKRecord) error {
	for name, destination := range map[string]*nullable.Nullable[string]{
		protocol.RemindersListFieldColorValue:        &result.Color,
		protocol.RemindersListFieldBadgeEmblemValue:  &result.BadgeEmblem,
		protocol.RemindersListFieldSortingStyleValue: &result.SortingStyle,
	} {
		err := projectReminderString(record, name, destination)
		if err != nil {
			return err
		}
	}

	err := projectReminderTitle(result, record)
	if err != nil {
		return err
	}

	count, err := reminderField(record, protocol.RemindersListFieldCountValue)
	if err != nil {
		return err
	}

	if len(count) != 0 && string(count) != jsonNullValue {
		result.Count, err = reminderCount(count)
		if err != nil {
			return err
		}
	}

	result.IsGroup, err = reminderListFlag(record, protocol.RemindersListFieldIsGroupValue)
	if err != nil {
		return err
	}

	result.Deleted, err = reminderListFlag(record, protocol.RemindersListFieldDeletedValue)

	return err
}

func projectReminderString(record cloudkit.CKRecord, name string, destination *nullable.Nullable[string]) error {
	raw, err := reminderField(record, name)
	if err != nil {
		return err
	}

	destination.SetNull()

	if len(raw) == 0 || string(raw) == jsonNullValue {
		return nil
	}

	if name == protocol.RemindersListFieldColorValue {
		truthy, truthErr := reminderTruthy(raw)
		if truthErr != nil {
			return truthErr
		}

		if !truthy {
			return nil
		}

		text, textErr := reminderDisplayText(raw)
		if textErr != nil {
			return textErr
		}

		destination.Set(text)

		return nil
	}

	err = json.Unmarshal(raw, destination)
	if err != nil {
		return fmt.Errorf("decode reminder list display field: %w", err)
	}

	return nil
}

func projectReminderTitle(result *ReminderList, record cloudkit.CKRecord) error {
	title, err := reminderField(record, protocol.RemindersListFieldNameValue)
	if err != nil {
		return err
	}

	truthy, err := reminderTruthy(title)
	if err != nil {
		return err
	}

	if !truthy {
		return nil
	}

	result.Title, err = reminderDisplayText(title)

	return err
}

func reminderCount(raw json.RawMessage) (int64, error) {
	var value int64

	err := json.Unmarshal(raw, &value)
	if err == nil {
		return value, nil
	}

	var text string

	err = json.Unmarshal(raw, &text)
	if err != nil {
		return 0, fmt.Errorf("decode reminder list count: %w", err)
	}

	value, err = strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse reminder list count: %w", err)
	}

	return value, nil
}

func reminderListFlag(record cloudkit.CKRecord, name string) (bool, error) {
	raw, err := reminderField(record, name)
	if err != nil {
		return false, err
	}

	return reminderTruthy(raw)
}
