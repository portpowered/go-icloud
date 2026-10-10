package photosync

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

var errSDKSelection = errors.New("photo sync account or library does not match the bound SDK session")

const (
	newSDKSourceOperation          = "NewSDKSource"
	applySessionResponsesOperation = "ApplySessionResponses"
)

// SDKClient is the public library subset used by local synchronization.
type SDKClient interface {
	ApplySessionResponses(ctx context.Context, request icloud.ApplySessionResponsesRequest) (
		*icloud.ApplySessionResponsesResult, error)
	GetPhotosCursor(ctx context.Context, request icloud.GetPhotosCursorRequest) (*icloud.GetPhotosCursorResult, error)
	VisitPhotoAssets(ctx context.Context, request icloud.ListPhotoAssetsRequest, visitor icloud.PhotoVisitor) (
		*icloud.ListPhotoAssetsResult, error)
	VisitRecentlyAddedPhotos(ctx context.Context,
		request icloud.ListRecentlyAddedPhotosRequest, visitor icloud.PhotoVisitor) (
		*icloud.ListRecentlyAddedPhotosResult, error)
	DownloadPhoto(ctx context.Context, request icloud.DownloadPhotoRequest) (*icloud.DownloadPhotoResult, error)
	DeletePhoto(ctx context.Context, request icloud.DeletePhotoRequest) (*icloud.PhotoDeletionResult, error)
}

// SDKSource owns an explicit copied account session; the reusable SDK stays stateless.
// Snapshot returns updated credentials and every consumed service response.
type SDKSource struct {
	client    SDKClient
	mu        sync.Mutex
	session   icloud.ResumeSessionResult
	revision  uint64
	libraries map[string]icloud.PhotoLibrary
}

// NewSDKSource binds SDK operations and libraries to one independent native session.
func NewSDKSource(ctx context.Context, client SDKClient, session icloud.ResumeSessionResult,
	libraries []icloud.PhotoLibrary,
) (*SDKSource, error) {
	if client == nil {
		return nil, &SyncError{Operation: newSDKSourceOperation, Cause: errConfiguration}
	}

	copied, err := client.ApplySessionResponses(ctx,
		icloud.ApplySessionResponsesRequest{Session: session, Responses: []icloud.ResponseMetadata{}})
	if err != nil {
		return nil, &SyncError{Operation: newSDKSourceOperation, Cause: err}
	}

	selected := make(map[string]icloud.PhotoLibrary, len(libraries))
	for _, library := range libraries {
		selected[library.ID] = cloneLibrary(library)
	}

	return &SDKSource{client: client, mu: sync.Mutex{}, session: copied.Session, revision: 0, libraries: selected}, nil
}

// Snapshot returns independent authentication state and ordered response evidence.
func (source *SDKSource) Snapshot(ctx context.Context) (*icloud.ResumeSessionResult, error) {
	source.mu.Lock()
	session := source.session
	source.mu.Unlock()

	result, err := source.client.ApplySessionResponses(ctx,
		icloud.ApplySessionResponsesRequest{Session: session, Responses: []icloud.ResponseMetadata{}})
	if err != nil {
		return nil, &SyncError{Operation: "Snapshot", Cause: err}
	}

	return &result.Session, nil
}

// Cursor reads the selected cursor and applies response rotations before returning.
func (source *SDKSource) Cursor(ctx context.Context, auth icloud.AuthContext, options Options) (*string, error) {
	current, library, err := source.selection(ctx, auth, options)
	if err != nil {
		return nil, &SyncError{Operation: "Cursor", Cause: err}
	}

	result, err := source.client.GetPhotosCursor(ctx, icloud.GetPhotosCursorRequest{Auth: current, Library: library})
	if err != nil {
		return nil, source.recordFailure(ctx, err, 0)
	}

	err = source.record(ctx, result.Responses)
	if err != nil {
		return nil, err
	}

	source.rememberCursor(options.Library, library, result.SyncToken)

	return &result.SyncToken, nil
}

// Visit streams photos and stops paging as soon as the visitor returns false.
func (source *SDKSource) Visit(ctx context.Context, auth icloud.AuthContext, options Options,
	visitor func(Asset) (bool, error),
) error {
	albums := normalizedAlbums(options.Albums)
	if len(albums) == 0 {
		albums = []string{"Library"}
	}

	for _, album := range albums {
		more, err := source.visitAlbum(ctx, auth, options, album, visitor)
		if err != nil {
			return err
		}

		if !more {
			return nil
		}
	}

	return nil
}

// Download retrieves a rendition using current credentials and preserves response evidence.
func (source *SDKSource) Download(ctx context.Context, auth icloud.AuthContext, asset Asset,
	key string,
) ([]byte, bool, error) {
	current, err := source.assetAuth(ctx, auth)
	if err != nil {
		return nil, false, err
	}

	version := icloud.PhotoVersion(key)

	album := stringValue(asset.Album)
	if album == "" {
		album = "Library"
	}

	result, err := source.client.DownloadPhoto(ctx, icloud.DownloadPhotoRequest{
		Auth: current, Album: album, Library: asset.Library, PhotoID: asset.ID, Version: &version})
	if err != nil {
		return nil, false, source.recordFailure(ctx, err, 0)
	}

	err = source.record(ctx, result.Responses)
	if err != nil {
		return nil, false, err
	}

	data, unavailable := result.Content.Get()

	return data, unavailable == nil, nil
}

// Delete soft-deletes a confirmed local asset, preserving acknowledgement and credential updates.
func (source *SDKSource) Delete(ctx context.Context, auth icloud.AuthContext, asset Asset) (bool, error) {
	current, err := source.assetAuth(ctx, auth)
	if err != nil {
		return false, err
	}

	result, err := source.client.DeletePhoto(ctx,
		icloud.DeletePhotoRequest{Auth: current, Album: asset.Album, Library: asset.Library, PhotoID: asset.ID})
	if err != nil {
		return false, source.recordFailure(ctx, err, 0)
	}

	err = source.record(ctx, result.Responses)
	if err != nil {
		return false, err
	}

	return result.Deleted, nil
}
func (source *SDKSource) selection(ctx context.Context, auth icloud.AuthContext, options Options) (
	icloud.AuthContext, *icloud.PhotoLibrary, error,
) {
	session, err := source.Snapshot(ctx)
	if err != nil {
		return auth, nil, err
	}

	if auth.AccountID != session.Auth.AccountID {
		return auth, nil, errSDKSelection
	}

	source.mu.Lock()
	library, exists := source.libraries[options.Library]
	source.mu.Unlock()

	if !exists {
		if options.Library == string(RootLibrary) {
			return session.Auth, nil, nil
		}

		return auth, nil, errSDKSelection
	}

	cloned := cloneLibrary(library)

	return session.Auth, &cloned, nil
}

func (source *SDKSource) rememberCursor(key string, library *icloud.PhotoLibrary, token string) {
	if library == nil {
		library = new(icloud.PhotoLibrary)
		library.ID, library.ZoneName = string(RootLibrary), protocol.PhotosPhotoPrimaryZoneNameValue
		library.ZoneType.Set(protocol.PhotosPhotoPrimaryZoneTypeValue)
		library.OwnerRecordName.SetNull()
	}

	copied := cloneLibrary(*library)
	copied.IndexingState = icloud.PhotoLibraryIndexingStateFINISHED
	copied.SyncToken.Set(token)
	source.mu.Lock()
	source.libraries[key] = copied
	source.mu.Unlock()
}
func cloneLibrary(library icloud.PhotoLibrary) icloud.PhotoLibrary {
	library.OwnerRecordName = maps.Clone(library.OwnerRecordName)
	library.ZoneType = maps.Clone(library.ZoneType)
	library.SyncToken = maps.Clone(library.SyncToken)

	return library
}
func (source *SDKSource) assetAuth(ctx context.Context, auth icloud.AuthContext) (icloud.AuthContext, error) {
	session, err := source.Snapshot(ctx)
	if err != nil {
		return auth, err
	}

	if auth.AccountID != session.Auth.AccountID {
		return auth, fmt.Errorf("photo account: %w", errSDKSelection)
	}

	return session.Auth, nil
}
func (source *SDKSource) record(ctx context.Context, responses []icloud.ResponseMetadata) error {
	for {
		err := source.recordOnce(ctx, responses)
		if !errors.Is(err, errSessionChanged) {
			return err
		}
	}
}

var errSessionChanged = errors.New("photo session changed during response application")

func (source *SDKSource) recordOnce(ctx context.Context, responses []icloud.ResponseMetadata) error {
	source.mu.Lock()
	session := source.session
	revision := source.revision
	source.mu.Unlock()

	result, err := source.client.ApplySessionResponses(context.WithoutCancel(ctx),
		icloud.ApplySessionResponsesRequest{Session: session, Responses: responses})
	if err != nil {
		return &SyncError{Operation: applySessionResponsesOperation, Cause: err}
	}

	source.mu.Lock()
	defer source.mu.Unlock()

	if source.revision != revision {
		return errSessionChanged
	}

	source.session = result.Session
	source.revision++

	return nil
}
func (source *SDKSource) recordFailure(ctx context.Context, cause error, observed int) error {
	var failure *icloud.ClientError
	if !errors.As(cause, &failure) {
		return cause
	}

	responses := failure.PriorResponses()
	if observed < len(responses) {
		responses = responses[observed:]
	} else {
		responses = nil
	}

	if failure.StatusCode() != 0 {
		responses = append(responses, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
			Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()})
	}

	err := source.record(ctx, responses)
	if err != nil {
		return &SyncError{Operation: applySessionResponsesOperation, Cause: errors.Join(cause, err)}
	}

	return cause
}
func (source *SDKSource) visitAlbum(ctx context.Context, auth icloud.AuthContext, options Options,
	album string, visitor func(Asset) (bool, error),
) (bool, error) {
	current, library, err := source.selection(ctx, auth, options)
	if err != nil {
		return false, &SyncError{Operation: "Visit", Cause: err}
	}

	state := &sdkVisit{source: source, album: album, library: library, visitor: visitor, observed: 0, more: true}
	accept := func(event icloud.PhotoVisitEvent) (bool, error) { return state.accept(ctx, event) }

	responses, err := source.enumerate(ctx, current, library, options, album, accept)
	if err != nil {
		return false, source.recordFailure(ctx, err, state.observed)
	}

	err = source.record(ctx, responses[state.observed:])
	if err != nil {
		return false, err
	}

	return state.more, nil
}
func (source *SDKSource) enumerate(ctx context.Context, auth icloud.AuthContext, library *icloud.PhotoLibrary,
	options Options, album string, visitor icloud.PhotoVisitor,
) ([]icloud.ResponseMetadata, error) {
	if len(normalizedAlbums(options.Albums)) == 0 && (options.Recent != nil || options.UntilFound != nil) {
		result, err := source.client.VisitRecentlyAddedPhotos(ctx,
			icloud.ListRecentlyAddedPhotosRequest{Auth: auth, Library: library}, visitor)
		if err != nil {
			return nil, fmt.Errorf("visit recently added: %w", err)
		}

		return result.Responses, nil
	}

	result, err := source.client.VisitPhotoAssets(ctx,
		icloud.ListPhotoAssetsRequest{Auth: auth, Library: library, Album: album}, visitor)
	if err != nil {
		return nil, fmt.Errorf("visit photo album: %w", err)
	}

	return result.Responses, nil
}

type sdkVisit struct {
	source   *SDKSource
	album    string
	library  *icloud.PhotoLibrary
	visitor  func(Asset) (bool, error)
	observed int
	more     bool
}

func (state *sdkVisit) accept(ctx context.Context, event icloud.PhotoVisitEvent) (bool, error) {
	err := state.source.record(ctx, event.Responses[state.observed:])
	if err != nil {
		return false, err
	}

	state.observed = len(event.Responses)

	asset, err := projectAsset(event.Photo, state.album, state.library)
	if err != nil {
		return false, err
	}

	state.more, err = state.visitor(asset)

	return state.more, err
}
