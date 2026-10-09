package webtransport

import (
	"encoding/json"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"reflect"
	"strings"
)

// Generated JSON tags supply required nested metadata identities. Unmarshal alone
// accepts absent required members, which must not make a union alternative valid.
func reminderModelRequired(model reflect.Type, fields map[string]json.RawMessage) bool {
	for index := range model.NumField() {
		field := model.Field(index)

		name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}

		raw, supplied := fields[name]
		if !reminderRequiredMember(field.Type, options, raw, supplied) {
			return false
		}

		if supplied && !reminderNestedRequired(field.Type, raw) {
			return false
		}
	}

	return true
}

func reminderRequiredMember(model reflect.Type, options string, raw json.RawMessage, supplied bool) bool {
	if !supplied {
		return options != ""
	}

	if string(raw) == jsonNullValue {
		// Optional non-nullable fields use pointers; nullable generated metadata
		// uses nullable.Nullable. An omitted default dictionary differs from null.
		return options != "" && model.Kind() != reflect.Pointer
	}

	return true
}

func reminderNestedRequired(model reflect.Type, raw json.RawMessage) bool {
	if model == reflect.TypeFor[cloudkit.CKIntegerInput]() {
		return reminderSyncInteger(raw)
	}

	if string(raw) == jsonNullValue {
		return true
	}

	for model.Kind() == reflect.Pointer || (model.Kind() == reflect.Map && model.Key().Kind() == reflect.Bool) {
		model = model.Elem()
	}

	if model.Kind() == reflect.Slice {
		return reminderArrayRequired(model.Elem(), raw)
	}

	if model.Kind() != reflect.Struct {
		return true
	}

	fields, err := accountFields(raw)

	return err == nil && reminderModelRequired(model, fields)
}

func reminderArrayRequired(model reflect.Type, raw json.RawMessage) bool {
	var values []json.RawMessage

	err := json.Unmarshal(raw, &values)
	if err != nil {
		return false
	}

	for _, value := range values {
		if !reminderNestedRequired(model, value) {
			return false
		}
	}

	return true
}
