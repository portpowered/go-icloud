package icloud

import (
	"context"
	"errors"
	"strconv"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

type photosRead struct {
	visitor       PhotoVisitor
	syncToken     *string
	sdk           *SDK
	auth          webtransport.RequestContext
	operation     string
	responses     []*webtransport.BytesResponse
	sharedLibrary bool
	libraries     []PhotoLibrary
}

func (sdk *SDK) beginPhotosRead(
	ctx context.Context,
	auth AuthContext,
	operation string,
	libraries ...*PhotoLibrary,
) (*photosRead, error) {
	err := ctx.Err()
	if err != nil {
		return nil, driveContextFailure(operation, err)
	}
	library := selectedPhotoLibrary(libraries)

	boundary, err := photosRequestContext(auth, library)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	boundary.Origin = auth.PhotosServiceURL

	read := &photosRead{
		sdk:           sdk,
		auth:          boundary,
		operation:     operation,
		responses:     []*webtransport.BytesResponse{},
		sharedLibrary: library != nil && library.IsSharedLibrary,
		libraries:     []PhotoLibrary{},
		syncToken:     nil,
		visitor:       nil,
	}

	if photoLibraryInitialized(library) {
		token, tokenErr := library.SyncToken.Get()
		if tokenErr == nil {
			read.syncToken = &token
		}

		return read, nil
	}

	response, err := sdk.web.PhotosIndexing(ctx, boundary)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	token, tokenErr := response.Data.SyncToken.Get()

	if tokenErr == nil {
		read.syncToken = &token
	}

	state, err := photosIndexingState(response.Data)
	if err != nil {
		return nil, read.failure(err, InvalidResponse)
	}

	if state != protocol.PhotosPhotoFinishedStateValue {
		return nil, read.failure(errPhotosIndexing, Unavailable)
	}

	return read, nil
}

func (read *photosRead) albums(ctx context.Context, parent *string) ([]cloudkit.CKRecord, error) {
	if read.sharedLibrary {
		return []cloudkit.CKRecord{}, nil
	}

	return read.privateAlbums(ctx, parent)
}

func (read *photosRead) privateAlbums(ctx context.Context, parent *string) ([]cloudkit.CKRecord, error) {
	records := []cloudkit.CKRecord{}

	var continuation *string

	for {
		response, err := read.sdk.web.PhotosAlbumQuery(ctx, read.auth, parent, continuation)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		read.responses = append(read.responses, response.Metadata)

		page, err := photoNormalRecords(response.Data)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		records = append(records, page...)

		next, _ := response.Data.ContinuationMarker.Get()
		if next == "" {
			break
		}

		continuation = &next
	}

	nested := []cloudkit.CKRecord{}

	for _, record := range records {
		raw, err := reminderField(record, protocol.PhotosPhotoAlbumTypeFieldValue)
		if err != nil {
			return nil, read.failure(err, InvalidResponse)
		}

		kind, numberErr := strconv.ParseFloat(string(raw), 64)
		if numberErr != nil || kind != float64(cloudkit.PhotoAlbumTypeN3) {
			continue
		}

		children, err := read.albums(ctx, &record.RecordName)
		if err != nil {
			return nil, err
		}

		nested = append(nested, children...)
	}

	return append(records, nested...), nil
}

func (read *photosRead) failure(err error, kind ErrorKind) *ClientError {
	var wire *webtransport.ResponseError
	if errors.As(err, &wire) {
		failure := adaptFailure(read.operation, err)
		for _, response := range read.responses {
			failure.prior = append(failure.prior, publicMetadata(response))
		}

		return failure
	}

	if len(read.responses) == 0 {
		return adaptFailure(read.operation, err)
	}

	last := read.responses[len(read.responses)-1]
	failure := newClientError(read.operation, kind, last.Status, last.Body, responseHeaders(last.Headers), err)

	failure.cookieScopeURL = last.CookieScopeURL

	for _, response := range read.responses[:len(read.responses)-1] {
		failure.prior = append(failure.prior, publicMetadata(response))
	}

	return failure
}

func (read *photosRead) metadata() []ResponseMetadata {
	result := make([]ResponseMetadata, 0, len(read.responses))
	for _, response := range read.responses {
		result = append(result, publicMetadata(response))
	}

	return result
}

func selectedPhotoLibrary(libraries []*PhotoLibrary) *PhotoLibrary {
	if len(libraries) == 0 {
		return nil
	}
	return libraries[0]
}
