package icloud

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

var errPhotosCursor = errors.New("no sync token available for photo library")

// GetPhotosCursor returns an explicit caller-owned current library cursor.
func (sdk *SDK) GetPhotosCursor(ctx context.Context, request GetPhotosCursorRequest) (*GetPhotosCursorResult, error) {
	read, err := sdk.beginPhotosRead(ctx, request.Auth, "GetPhotosCursor", request.Library)
	if err != nil {
		return nil, err
	}

	if read.syncToken != nil && *read.syncToken != "" {
		return &GetPhotosCursorResult{SyncToken: *read.syncToken, Responses: read.metadata()}, nil
	}

	response, err := sdk.web.PhotosLibraryZones(ctx, read.auth, read.auth.PhotoShared)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	name := protocol.PhotosPhotoPrimaryZoneNameValue
	if request.Library != nil {
		name = request.Library.ZoneName
	}

	if response.Data.Zones != nil {
		for _, zone := range *response.Data.Zones {
			if zone.ZoneID.ZoneName == name {
				token, _ := zone.SyncToken.Get()
				if token != "" {
					return &GetPhotosCursorResult{SyncToken: token, Responses: read.metadata()}, nil
				}

				break
			}
		}
	}

	return nil, read.failure(errPhotosCursor, Unavailable)
}

// GetPhotoChanges consumes every change page and returns its final cursor without retaining account state.
func (sdk *SDK) GetPhotoChanges(ctx context.Context, request GetPhotoChangesRequest) (*GetPhotoChangesResult, error) {
	read, err := sdk.beginPhotosRead(ctx, request.Auth, "GetPhotoChanges", request.Library)
	if err != nil {
		return nil, err
	}

	result := &GetPhotoChangesResult{Changes: []PhotoChange{}, SyncToken: nil, Responses: []ResponseMetadata{}}
	result.SyncToken.SetNull()

	since := request.Since

	for {
		page, pageErr := sdk.web.PhotosChanges(ctx, read.auth, since)
		if pageErr != nil {
			return nil, read.failure(pageErr, InvalidResponse)
		}

		read.responses = append(read.responses, page.Metadata)

		if page.Data.Zones == nil || len(*page.Data.Zones) == 0 {
			break
		}

		zone := (*page.Data.Zones)[0]
		result.SyncToken.Set(zone.SyncToken)

		changes, changeErr := projectPhotoChanges(zone)
		if changeErr != nil {
			return nil, read.failure(changeErr, InvalidResponse)
		}

		result.Changes = append(result.Changes, changes...)

		more, _ := zone.MoreComing.Get()
		if !more {
			break
		}

		token := zone.SyncToken
		since = &token
	}

	result.Responses = read.metadata()

	return result, nil
}

func projectPhotoChanges(zone cloudkit.CKZoneChangesZone) ([]PhotoChange, error) {
	changes := []PhotoChange{}
	if zone.Records == nil {
		return changes, nil
	}

	for _, item := range *zone.Records {
		record, err := webtransport.DecodeReminderEventRecord(item)
		if err != nil {
			return nil, fmt.Errorf("decode photo change record: %w", err)
		}

		change := PhotoChange{Kind: PhotoUpdated, RecordName: "", RecordType: nil, Deleted: false, Modified: nil}
		change.RecordType.SetNull()
		change.Modified.SetNull()

		switch {
		case record.Tombstone != nil:
			change.Kind = PhotoDeleted
			change.RecordName = record.Tombstone.RecordName
			change.Deleted = true
		case record.Record != nil:
			change.RecordName = record.Record.RecordName
			change.RecordType.Set(record.Record.RecordType)
			value, _ := record.Record.Deleted.Get()

			change.Deleted = value

			modified, modifiedErr := record.Record.Modified.Get()

			if modifiedErr == nil {
				change.Modified.Set(time.UnixMilli(modified.Timestamp).UTC())
			}
		default:
			continue
		}

		changes = append(changes, change)
	}

	return changes, nil
}
