package webtransport

import (
	"encoding/json"
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
		if options == "" && (!supplied || string(raw) == jsonNullValue) {
			return false
		}

		if supplied && !reminderNestedRequired(field.Type, raw) {
			return false
		}
	}

	return true
}

func reminderNestedRequired(model reflect.Type, raw json.RawMessage) bool {
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
