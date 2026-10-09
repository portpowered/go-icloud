package webtransport

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func validReminderEncryptedField(name string, field cloudkit.CKFieldOpen) bool {
	if !strings.HasSuffix(name, protocol.RemindersEncryptedFieldNameSuffixValue) {
		return true
	}

	value, err := field.AsCKPassthroughField()
	if err != nil {
		return false
	}

	if value.Type == string(cloudkit.ENCRYPTEDBYTES) {
		return true
	}

	if value.Type != string(cloudkit.CKStringFieldTypeSTRING) {
		return false
	}

	return reminderEncryptedTrue(value.AdditionalProperties[protocol.RemindersCKStringFieldIsEncrypted])
}

func reminderEncryptedTrue(raw json.RawMessage) bool {
	if string(raw) == jsonTrueValue {
		return true
	}

	var text string
	if json.Unmarshal(raw, &text) == nil {
		valid, err := regexp.MatchString(protocol.RemindersCKBooleanTrueInputTextPattern, text)

		return err == nil && valid
	}

	var number float64

	return json.Unmarshal(raw, &number) == nil && number == 1
}
