package icloud

import (
	"context"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// ListReminderZones discovers fresh reminder zones using caller-owned authentication.
func (sdk *SDK) ListReminderZones(ctx context.Context,
	request ListReminderZonesRequest,
) (*ListReminderZonesResult, error) {
	const operation = "ListReminderZones"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	boundary.Origin = request.Auth.RemindersServiceURL

	response, err := sdk.web.ListReminderZones(ctx, boundary)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	zones := make([]ReminderZone, 0)

	if response.Data.Zones != nil {
		for _, zone := range *response.Data.Zones {
			zones = append(zones, projectReminderZone(zone))
		}
	}

	return &ListReminderZonesResult{Zones: zones, Metadata: publicMetadata(response.Metadata)}, nil
}

func projectReminderZone(zone cloudkit.CKZoneListZone) ReminderZone {
	result := ReminderZone{Name: zone.ZoneID.ZoneName,
		Owner: zone.ZoneID.OwnerRecordName, Type: zone.ZoneID.ZoneType,
		SyncToken: zone.SyncToken, Deleted: zone.Deleted}
	if !result.Owner.IsSpecified() {
		result.Owner.SetNull()
	}

	if !result.Type.IsSpecified() {
		result.Type.SetNull()
	}

	if !result.SyncToken.IsSpecified() {
		result.SyncToken.SetNull()
	}

	if !result.Deleted.IsSpecified() {
		result.Deleted.SetNull()
	}

	return result
}
