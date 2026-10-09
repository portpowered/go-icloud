package icloud

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const reminderLinkedOperationCount = 2

func reminderCopiedNullable[Value any](input nullable.Nullable[Value]) nullable.Nullable[Value] {
	result := maps.Clone(input)
	if !result.IsSpecified() {
		result.SetNull()
	}

	return result
}

func reminderCopiedIDs(input []string) []string {
	return append([]string{}, input...)
}

func reminderRequiredTimestamp(now time.Time) cloudkit.ReminderWriteTimestamp {
	return cloudkit.ReminderWriteTimestamp{Type: cloudkit.ReminderWriteTimestampTypeTIMESTAMP,
		Value: reminderTimestamp(now).Value}
}

func reminderWriteContext(ctx context.Context, input AuthContext,
	operation string,
) (webtransport.RequestContext, error) {
	err := ctx.Err()
	if err != nil {
		return webtransport.RequestContext{}, driveContextFailure(operation, err)
	}

	auth, err := accountRequestContext(input)
	if err != nil {
		return webtransport.RequestContext{}, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = input.RemindersServiceURL

	return auth, nil
}

func reminderAcknowledgements(operation string,
	response *webtransport.ReminderModificationResponse,
) (map[string]string, error) {
	tags := make(map[string]string)
	if response.Data.Records == nil {
		return tags, nil
	}

	for _, item := range *response.Data.Records {
		selected, err := webtransport.DecodeReminderModificationRecord(item)
		if err != nil {
			return nil, reminderModificationFailure(operation, InvalidResponse, err, response)
		}

		if selected.Failure != nil {
			cause := fmt.Errorf("%w: %s", errReminderList, selected.Failure.ServerErrorCode)

			return nil, reminderModificationFailure(operation, Provider, cause, response)
		}

		if selected.Record != nil && selected.Record.RecordChangeTag.GetOrEmpty() != "" {
			tags[selected.Record.RecordName] = selected.Record.RecordChangeTag.GetOrEmpty()
		}
	}

	return tags, nil
}

func reminderModificationFailure(operation string, kind ErrorKind, cause error,
	response *webtransport.ReminderModificationResponse,
) *ClientError {
	metadata := response.Metadata
	failure := newClientError(operation, kind, metadata.Status, metadata.Body,
		responseHeaders(metadata.Headers), cause)
	failure.cookieScopeURL = metadata.CookieScopeURL

	return failure
}

func reminderAcknowledgedTag(input nullable.Nullable[string], name string, tags map[string]string,
) nullable.Nullable[string] {
	result := maps.Clone(input)
	if tag := tags[name]; tag != "" {
		result.Set(tag)
	} else if !result.IsSpecified() {
		result.SetNull()
	}

	return result
}

func reminderRevisionPointer(input nullable.Nullable[string]) *string {
	if !input.IsSpecified() || input.IsNull() {
		return nil
	}

	value := input.GetOrEmpty()

	return &value
}

func reminderRelatedRecordName(name, prefix string) string {
	if name == "" || strings.HasPrefix(name, prefix) {
		return name
	}

	return prefix + name
}

func reminderRelatedIDs(input []string, prefix, removed string) []string {
	result := make([]string, 0, len(input))

	for _, name := range input {
		raw := strings.TrimPrefix(name, prefix)
		if removed == "" || raw != removed {
			result = append(result, raw)
		}
	}

	return result
}
