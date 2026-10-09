package webtransport

import (
	"encoding/json"
	"reflect"
	"strings"
)

const reminderUnionRankLevels = 2

func reminderUnionRank(model reflect.Type, raw json.RawMessage, exact bool) int {
	rank := reminderUnionScore(model, raw) * reminderUnionRankLevels
	if exact {
		rank++
	}

	return rank
}

// Count only declared, supplied model fields, including nested model members.
// Source smart unions ignore extra fields and prefer the earlier model on ties.
// Strict matches win over coerced matches when their supplied-field counts tie.
// Generated model tags own this inventory; free-form dictionaries do not add members.
func reminderUnionScore(model reflect.Type, raw json.RawMessage) int {
	if string(raw) == jsonNullValue {
		return 0
	}

	for model.Kind() == reflect.Pointer || (model.Kind() == reflect.Map && model.Key().Kind() == reflect.Bool) {
		model = model.Elem()
	}

	if model.Kind() == reflect.Slice {
		return reminderUnionArrayScore(model.Elem(), raw)
	}

	if model.Kind() != reflect.Struct {
		return 0
	}

	return reminderUnionObjectScore(model, raw)
}

func reminderUnionObjectScore(model reflect.Type, raw json.RawMessage) int {
	fields, err := accountFields(raw)
	if err != nil {
		return 0
	}

	score := 0

	for index := range model.NumField() {
		field := model.Field(index)
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")

		value, supplied := fields[name]

		if name == "" || name == "-" || !supplied {
			continue
		}

		score += 1 + reminderUnionScore(field.Type, value)
	}

	return score
}

func reminderUnionArrayScore(model reflect.Type, raw json.RawMessage) int {
	var values []json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return 0
	}

	score := 0
	for _, value := range values {
		score += reminderUnionScore(model, value)
	}

	return score
}
