package icloud

import (
	"context"
)

// GetPhotoLibraryChanges reads a caller-selected database page and returns the next cursor.
func (sdk *SDK) GetPhotoLibraryChanges(
	ctx context.Context,
	request GetPhotoLibraryChangesRequest,
) (*GetPhotoLibraryChangesResult, error) {
	const operation = "GetPhotoLibraryChanges"

	read, err := sdk.beginPhotosRead(ctx, request.Auth, operation)
	if err != nil {
		return nil, err
	}

	read.auth.PhotoShared = request.Shared

	response, err := sdk.web.PhotosDatabaseChanges(ctx, read.auth, request.Since)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	result := &GetPhotoLibraryChangesResult{
		Zones:      []PhotoLibraryChange{},
		SyncToken:  response.Data.SyncToken,
		MoreComing: response.Data.MoreComing,
		Responses:  read.metadata(),
	}
	if !result.SyncToken.IsSpecified() {
		result.SyncToken.SetNull()
	}

	if !result.MoreComing.IsSpecified() {
		result.MoreComing.SetNull()
	}

	if response.Data.Zones != nil {
		for _, zone := range *response.Data.Zones {
			change := PhotoLibraryChange{
				ZoneName:        zone.ZoneID.ZoneName,
				ZoneType:        zone.ZoneID.ZoneType,
				OwnerRecordName: zone.ZoneID.OwnerRecordName,
				Deleted:         zone.Deleted,
			}
			if !change.ZoneType.IsSpecified() {
				change.ZoneType.SetNull()
			}

			if !change.OwnerRecordName.IsSpecified() {
				change.OwnerRecordName.SetNull()
			}

			if !change.Deleted.IsSpecified() {
				change.Deleted.SetNull()
			}

			result.Zones = append(result.Zones, change)
		}
	}

	return result, nil
}
