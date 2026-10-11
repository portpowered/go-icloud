package icloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

var (
	errPhotoRecordLookup  = errors.New("photo record lookup failed")
	errPhotoRecordMissing = errors.New("photo record is absent")
)

func (read *photosRead) lookupLibraryPhotoRecords(ctx context.Context, photoID string,
	project photoProjection,
) ([]Photo, error) {
	asset, err := read.lookupPhotoRecord(ctx, photoID)
	if errors.Is(err, errPhotoRecordMissing) {
		return []Photo{}, nil
	}

	if err != nil || asset == nil {
		return []Photo{}, err
	}

	if asset.RecordType != string(cloudkit.CPLAsset) {
		return nil, read.failure(errPhotoRecordLookup, InvalidResponse)
	}

	for _, field := range []string{"isDeleted", "isHidden"} {
		raw, fieldErr := photoFieldValue(*asset, field)
		if fieldErr != nil {
			return nil, read.failure(fieldErr, InvalidResponse)
		}

		if len(raw) != 0 && string(raw) != "null" {
			var flag int64

			if json.Unmarshal(raw, &flag) != nil {
				return nil, read.failure(errPhotoRecordLookup, InvalidResponse)
			}

			if flag != 0 {
				return []Photo{}, nil
			}
		}
	}

	masterID, err := reminderReference(*asset, string(cloudkit.MasterRef))
	if err != nil || masterID == "" {
		return nil, read.failure(errPhotoRecordLookup, InvalidResponse)
	}

	master, err := read.lookupPhotoRecord(ctx, masterID)
	if errors.Is(err, errPhotoRecordMissing) {
		return []Photo{}, nil
	}

	if err != nil || master == nil {
		return []Photo{}, err
	}

	if master.RecordType != string(cloudkit.CPLMaster) {
		return nil, read.failure(errPhotoRecordLookup, InvalidResponse)
	}

	photo, err := project(*master, *asset)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	return []Photo{photo}, nil
}

func (read *photosRead) lookupPhotoRecord(ctx context.Context, name string) (*cloudkit.CKRecord, error) {
	response, err := read.sdk.web.LookupPhotosRecords(ctx, read.auth, []string{name})
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	for _, item := range response.Data.Records {
		selected, decodeErr := webtransport.DecodeReminderLookupRecord(item)
		if decodeErr != nil {
			return nil, read.failure(decodeErr, InvalidResponse)
		}

		if selected.Failure != nil {
			if selected.Failure.RecordName.GetOrEmpty() != name {
				return nil, read.failure(errPhotoRecordLookup, InvalidResponse)
			}

			if selected.Failure.ServerErrorCode == "UNKNOWN_ITEM" {
				return nil, errPhotoRecordMissing
			}

			return nil, read.failure(fmt.Errorf("%w: %s", errPhotoRecordLookup,
				selected.Failure.ServerErrorCode), Unavailable)
		}

		if selected.Record != nil && selected.Record.RecordName == name {
			return selected.Record, nil
		}
	}

	return nil, read.failure(errPhotoRecordLookup, InvalidResponse)
}
