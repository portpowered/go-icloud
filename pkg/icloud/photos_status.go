package icloud

import (
	"context"
	"errors"
	"fmt"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

const photosStatusOperation = "GetPhotosStatus"

var errPhotosIndexing = errors.New("photo library indexing is not finished")

// GetPhotosStatus initializes the primary photo library and retains its current cursor.
// A library that has not finished indexing returns an Unavailable error with response evidence.
func (sdk *SDK) GetPhotosStatus(ctx context.Context, request GetPhotosStatusRequest) (*GetPhotosStatusResult, error) {
	auth, err := photosRequestContext(request.Auth, request.Library)
	if err != nil {
		return nil, newClientError(photosStatusOperation, Configuration, 0, nil, nil, err)
	}

	auth.Origin = request.Auth.PhotosServiceURL

	response, err := sdk.web.PhotosIndexing(ctx, auth)
	if err != nil {
		return nil, adaptFailure(photosStatusOperation, err)
	}

	state, err := photosIndexingState(response.Data)
	if err != nil {
		return nil, photosStatusFailure(InvalidResponse, err, response.Metadata)
	}

	if state != protocol.PhotosPhotoFinishedStateValue {
		return nil, photosStatusFailure(Unavailable, errPhotosIndexing, response.Metadata)
	}

	result := &GetPhotosStatusResult{State: GetPhotosStatusResultState(state), SyncToken: response.Data.SyncToken,
		Metadata: publicMetadata(response.Metadata)}
	if !result.SyncToken.IsSpecified() {
		result.SyncToken.SetNull()
	}

	return result, nil
}

func photosIndexingState(data cloudkit.CKQueryResponse) (string, error) {
	if data.Records == nil {
		return "", nil
	}

	for _, item := range *data.Records {
		record, err := webtransport.DecodeReminderQueryRecord(item)
		if err != nil {
			return "", fmt.Errorf("decode photo indexing record: %w", err)
		}

		if record.Record == nil {
			continue
		}

		return photosIndexingStateText(*record.Record)
	}

	return "", nil
}

func photosStatusFailure(kind ErrorKind, err error, response *webtransport.BytesResponse) *ClientError {
	failure := newClientError(photosStatusOperation, kind, response.Status, response.Body,
		responseHeaders(response.Headers), err)
	failure.cookieScopeURL = response.CookieScopeURL

	return failure
}

func photosIndexingStateText(record cloudkit.CKRecord) (string, error) {
	if record.Fields != nil {
		if field, exists := (*record.Fields)[protocol.PhotosPhotoIndexingStateFieldValue]; exists {
			wrapper, err := field.AsCKPassthroughField()
			if err != nil {
				return "", fmt.Errorf("decode photo indexing field: %w", err)
			}

			switch wrapper.Type {
			case string(cloudkit.BYTES), string(cloudkit.ENCRYPTEDBYTES):
				// Source converts binary values to Python byte representations, never the text readiness token.
				return "", nil
			}
		}
	}

	raw, err := reminderField(record, protocol.PhotosPhotoIndexingStateFieldValue)
	if err != nil {
		return "", err
	}

	if len(raw) == 0 || string(raw) == jsonNullValue {
		return "", nil
	}

	return reminderDisplayText(raw)
}
