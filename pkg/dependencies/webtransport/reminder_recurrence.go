package webtransport

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// CreateReminderRecurrence sends an atomic reminder link and recurrence creation.
func (client *Client) CreateReminderRecurrence(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderRecurrenceCreationRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}

// UpdateReminderRecurrence sends a revision-aware recurrence update.
func (client *Client) UpdateReminderRecurrence(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderRecurrenceUpdateRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}

// DeleteReminderRecurrence sends an atomic unlink and soft deletion.
func (client *Client) DeleteReminderRecurrence(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderRecurrenceDeletionRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}
