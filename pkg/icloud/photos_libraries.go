package icloud

import (
	"context"
	"errors"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func (read *photosRead) discoverPhotoLibraries(ctx context.Context) error {
	seen := map[string]bool{}

	err := read.initializePhotoScope(ctx, false, seen)
	if err != nil {
		return err
	}

	err = read.initializePhotoScope(ctx, true, seen)
	if err == nil {
		return nil
	}

	response := webtransport.SuppressedPhotosResponse(err)
	if response != nil {
		read.responses = append(read.responses, response)

		return nil
	}

	var failure *ClientError
	if errors.As(err, &failure) && (failure.Kind() == Unavailable || failure.Kind() == InvalidResponse) {
		return nil
	}

	return err
}

func (read *photosRead) initializePhotoScope(ctx context.Context, shared bool, seen map[string]bool) error {
	page, err := read.sdk.web.PhotosLibraryZones(ctx, read.auth, shared)
	if err != nil {
		return read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, page.Metadata)

	if page.Data.Zones == nil {
		return nil
	}

	for _, zone := range *page.Data.Zones {
		deleted, _ := zone.Deleted.Get()
		if deleted || (!shared && zone.ZoneID.ZoneName == protocol.PhotosPhotoPrimaryZoneNameValue) {
			continue
		}

		key := photoLibraryKey(zone.ZoneID.ZoneName, shared)
		if shared && seen[key] {
			continue
		}

		err = read.initializePhotoLibrary(ctx, zone.ZoneID, shared)
		if err != nil {
			return err
		}

		seen[key] = true
	}

	return nil
}

func photoLibraryKey(name string, shared bool) string {
	if shared || strings.HasPrefix(name, protocol.PhotosPhotoSharedLibraryZonePrefixValue) {
		return protocol.PhotosPhotoSharedLibraryKeyPrefixValue + name
	}

	return name
}

func (read *photosRead) initializePhotoLibrary(ctx context.Context, zone cloudkit.CKZoneID, shared bool) error {
	response, err := read.sdk.web.PhotosLibraryIndexing(ctx, read.auth, zone, shared)
	if err != nil {
		return read.failure(err, InvalidResponse)
	}

	read.responses = append(read.responses, response.Metadata)

	state, err := photosIndexingState(response.Data)
	if err != nil {
		return read.failure(err, InvalidResponse)
	}

	if state != protocol.PhotosPhotoFinishedStateValue {
		return read.failure(errPhotosIndexing, Unavailable)
	}

	return nil
}
