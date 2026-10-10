package webtransport

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// AddReminderLocationTrigger sends the atomic generated parent, alarm and trigger operations.
func (client *Client) AddReminderLocationTrigger(ctx context.Context, auth RequestContext,
	input cloudkit.ReminderLocationRequest,
) (*ReminderModificationResponse, error) {
	body, err := referenceJSON(input)

	return client.modifyReminder(ctx, auth, body, err)
}
