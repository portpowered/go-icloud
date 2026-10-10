package webtransport

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ModifyReminderAttachment sends a schema-owned attachment mutation.
func (client *Client) ModifyReminderAttachment(ctx context.Context, auth RequestContext,
	input cloudkit.RemindersModificationRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}
