package icloud

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

// Client provides the currently implemented iCloud operations.
// Credentials belong to each request; a Client can serve multiple accounts.
//
//nolint:interfacebloat // API-01: trace selected operations on one Client.
type Client interface {
	// GetPhoto finds one album asset by identifier with Source enumeration fallback.
	GetPhoto(ctx context.Context, request GetPhotoRequest) (*GetPhotoResult, error)
	// GetLegacyRemindersSnapshot reads account-discovered legacy startup lists and reminders.
	GetLegacyRemindersSnapshot(ctx context.Context,
		request GetLegacyRemindersSnapshotRequest,
	) (*GetLegacyRemindersSnapshotResult, error)
	// ListPhotoAssets enumerates paired assets of a primary photo album.
	ListPhotoAssets(ctx context.Context, request ListPhotoAssetsRequest) (*ListPhotoAssetsResult, error)
	// GetPhotoAlbumCount reads the indexed count of a primary-library album.
	GetPhotoAlbumCount(ctx context.Context, request GetPhotoAlbumCountRequest) (*GetPhotoAlbumCountResult, error)
	// ListPhotoAlbums reads all primary-library smart and custom albums.
	ListPhotoAlbums(ctx context.Context, request ListPhotoAlbumsRequest) (*ListPhotoAlbumsResult, error)
	// GetPhotosStatus checks primary photo library readiness and its current cursor.
	GetPhotosStatus(ctx context.Context, request GetPhotosStatusRequest) (*GetPhotosStatusResult, error)
	// ListReminderSnapshot collects reminders across discovered lists or one optional list filter.
	ListReminderSnapshot(ctx context.Context, request ListReminderSnapshotRequest) (*ListReminderSnapshotResult, error)
	// ListReminderTags reads hashtags using raw or full related identifiers.
	ListReminderTags(ctx context.Context, request ListReminderTagsRequest) (*ListReminderTagsResult, error)
	// ListReminderAttachments reads supported URL and image attachments.
	ListReminderAttachments(ctx context.Context,
		request ListReminderAttachmentsRequest,
	) (*ListReminderAttachmentsResult, error)
	// ListReminderRecurrenceRules reads ordered recurrence rules.
	ListReminderRecurrenceRules(ctx context.Context,
		request ListReminderRecurrenceRulesRequest,
	) (*ListReminderRecurrenceRulesResult, error)
	// ListReminderAlarms reads ordered alarms and resolves their location triggers.
	ListReminderAlarms(ctx context.Context, request ListReminderAlarmsRequest) (*ListReminderAlarmsResult, error)
	// ListReminders reads a complete list snapshot with scoped related records.
	ListReminders(ctx context.Context, request ListRemindersRequest) (*ListRemindersResult, error)
	// ListReminderChanges consumes ordered reminder updates and deletions from an optional cursor.
	ListReminderChanges(ctx context.Context, request ListReminderChangesRequest) (*ListReminderChangesResult, error)
	// GetReminderSyncCursor discovers a usable token, consuming fallback pages when required.
	GetReminderSyncCursor(ctx context.Context, request GetReminderSyncCursorRequest) (*GetReminderSyncCursorResult, error)
	// GetReminder reads one complete reminder by raw or full record identifier.
	GetReminder(ctx context.Context, request GetReminderRequest) (*GetReminderResult, error)
	// ListReminderLists reads the complete ordered list snapshot and membership.
	ListReminderLists(ctx context.Context, request ListReminderListsRequest) (*ListReminderListsResult, error)
	// ListReminderZones discovers reminder storage zones and change cursors.
	ListReminderZones(ctx context.Context, request ListReminderZonesRequest) (*ListReminderZonesResult, error)
	// ResumeSession validates and refreshes caller-owned saved web credentials.
	ResumeSession(ctx context.Context, request ResumeSessionRequest) (*ResumeSessionResult, error)
	// OpenFindMySession discovers devices and binds cache, polling and commands to one account.
	OpenFindMySession(ctx context.Context, request OpenFindMySessionRequest,
		options ...FindMyOption,
	) (*FindMySession, error)
	// OpenDriveSession binds a cache and credential lifecycle to one copied account context.
	OpenDriveSession(ctx context.Context, request OpenDriveSessionRequest) (*DriveSession, error)
	// UploadDriveFile prepares, transfers and registers caller-owned seekable content.
	UploadDriveFile(ctx context.Context, request UploadDriveFileRequest) (*UploadDriveFileResult, error)
	// DownloadDriveFile retrieves exact document bytes through a provider-issued content URL.
	DownloadDriveFile(ctx context.Context, request DownloadDriveFileRequest) (*DownloadDriveFileResult, error)
	// CreateDriveFolder creates one named folder.
	CreateDriveFolder(ctx context.Context, request CreateDriveFolderRequest) (*CreateDriveFolderResult, error)
	// RenameDriveNode requests a new node name using its supplied version token.
	RenameDriveNode(ctx context.Context, request RenameDriveNodeRequest) (*RenameDriveNodeResult, error)
	// MoveDriveNodes requests movement of the selected nodes, including an empty selection.
	MoveDriveNodes(ctx context.Context, request MoveDriveNodesRequest) (*MoveDriveNodesResult, error)
	// TrashDriveNode requests movement of a selected node into trash.
	TrashDriveNode(ctx context.Context, request TrashDriveNodeRequest) (*TrashDriveNodeResult, error)
	// RestoreDriveNode requests recovery of a selected trash node.
	RestoreDriveNode(ctx context.Context, request RestoreDriveNodeRequest) (*RestoreDriveNodeResult, error)
	// DeleteDriveNode requests ordinary deletion of a selected node.
	DeleteDriveNode(ctx context.Context, request DeleteDriveNodeRequest) (*DeleteDriveNodeResult, error)
	// PermanentlyDeleteDriveNode requests permanent deletion of a selected trash node.
	PermanentlyDeleteDriveNode(ctx context.Context,
		request PermanentlyDeleteDriveNodeRequest,
	) (*PermanentlyDeleteDriveNodeResult, error)
	// GetDriveNode returns fresh node metadata, including available folder contents.
	GetDriveNode(ctx context.Context, request GetDriveNodeRequest) (*GetDriveNodeResult, error)
	// ListDriveLibraries returns fresh application-library records.
	ListDriveLibraries(ctx context.Context, request ListDriveLibrariesRequest) (*ListDriveLibrariesResult, error)
	// GetAccountDevices returns fresh devices and payment-method metadata.
	GetAccountDevices(ctx context.Context, request GetAccountDevicesRequest) (*GetAccountDevicesResult, error)
	// GetAccountFamily returns fresh family records.
	GetAccountFamily(ctx context.Context, request GetAccountFamilyRequest) (*GetAccountFamilyResult, error)
	// GetAccountMemberPhoto returns exact photo bytes for a member DSID.
	GetAccountMemberPhoto(ctx context.Context, request GetAccountMemberPhotoRequest) (*GetAccountMemberPhotoResult, error)
	// GetAccountStorage returns fresh usage, quota and media metadata.
	GetAccountStorage(ctx context.Context, request GetAccountStorageRequest) (*GetAccountStorageResult, error)
	// GetAccountPlanSummary returns opaque JSON from the account's regional gateway.
	GetAccountPlanSummary(ctx context.Context, request GetAccountPlanSummaryRequest) (*GetAccountPlanSummaryResult, error)
}

var (
	errTransportConfig = errors.New("HTTP transport must be nonnil and configured once")
	errNilOption       = errors.New("nil client option")
	errAuthIdentifiers = errors.New("account and client identifiers are required")
	errMemberID        = errors.New("family member identifier is required")
	errDriveNodeID     = errors.New("drive node identifier is required")
)

type configuration struct {
	transport  http.RoundTripper
	configured bool
	clock      func() time.Time
}

// Option configures a reusable client without storing account credentials.
type Option func(*configuration) error

// WithHTTPTransport injects a caller-owned transport safe for concurrent requests.
// The SDK owns its HTTP client and never shares an account cookie jar.
func WithHTTPTransport(transport http.RoundTripper) Option {
	return func(config *configuration) error {
		if transport == nil || config.configured {
			return errTransportConfig
		}

		config.transport = transport
		config.configured = true

		return nil
	}
}

// SDK implements Client with immutable transport configuration.
type SDK struct {
	web   *webtransport.Client
	clock func() time.Time
}

// New creates a reusable stateless client. The caller supplies request deadlines.
// The default is http.DefaultTransport; automatic redirects are disabled.
func New(options ...Option) (*SDK, error) {
	config := configuration{transport: http.DefaultTransport, configured: false, clock: time.Now}

	for _, option := range options {
		if option == nil {
			return nil, newClientError("New", Configuration, 0, nil, nil, errNilOption)
		}

		err := option(&config)
		if err != nil {
			return nil, newClientError("New", Configuration, 0, nil, nil, err)
		}
	}

	return &SDK{web: webtransport.New(config.transport), clock: config.clock}, nil
}

// GetAccountDevices fetches fresh account devices using caller-owned authentication state.
func (sdk *SDK) GetAccountDevices(ctx context.Context,
	request GetAccountDevicesRequest,
) (*GetAccountDevicesResult, error) {
	const operation = "GetAccountDevices"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.GetDevices(ctx, boundary)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDevices(response), nil
}

// GetAccountFamily fetches fresh members using caller-owned account authentication.
func (sdk *SDK) GetAccountFamily(ctx context.Context,
	request GetAccountFamilyRequest,
) (*GetAccountFamilyResult, error) {
	const operation = "GetAccountFamily"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.GetFamily(ctx, boundary)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectFamily(response), nil
}

// GetAccountMemberPhoto fetches the photo for the DSID returned by GetAccountFamily.
func (sdk *SDK) GetAccountMemberPhoto(ctx context.Context,
	request GetAccountMemberPhotoRequest,
) (*GetAccountMemberPhotoResult, error) {
	const operation = "GetAccountMemberPhoto"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	if request.MemberID == "" {
		return nil, newClientError(operation, Configuration, 0, nil, nil, errMemberID)
	}

	response, err := sdk.web.GetMemberPhoto(ctx, boundary, request.MemberID)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return &GetAccountMemberPhotoResult{Content: response.Body, Metadata: publicMetadata(response)}, nil
}

// GetAccountStorage fetches absolute byte counts and available quota/media metadata.
func (sdk *SDK) GetAccountStorage(ctx context.Context,
	request GetAccountStorageRequest,
) (*GetAccountStorageResult, error) {
	const operation = "GetAccountStorage"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.GetStorage(ctx, boundary)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectStorage(response), nil
}

// GetAccountPlanSummary fetches opaque subscription JSON from the authenticated account region.
func (sdk *SDK) GetAccountPlanSummary(ctx context.Context,
	request GetAccountPlanSummaryRequest,
) (*GetAccountPlanSummaryResult, error) {
	const operation = "GetAccountPlanSummary"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	boundary.Origin = protocol.AccountServer1
	if request.Auth.ChinaMainland != nil && *request.Auth.ChinaMainland {
		boundary.Origin = protocol.AccountServer2
	}

	response, err := sdk.web.GetPlanSummary(ctx, boundary)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return &GetAccountPlanSummaryResult{Summary: response.Body, Metadata: publicMetadata(response)}, nil
}

// DownloadDriveFile first locates content using the account's document-service origin.
// The caller owns the returned token-response and content-response authentication updates.
func (sdk *SDK) DownloadDriveFile(ctx context.Context,
	request DownloadDriveFileRequest,
) (*DownloadDriveFileResult, error) {
	const operation = "DownloadDriveFile"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	boundary.Origin = request.Auth.DriveDocumentServiceURL
	boundary.DriveToken = request.Auth.DriveToken

	zone := string(drive.ComAppleCloudDocs)
	if request.Zone != nil {
		zone = *request.Zone
	}

	response, err := sdk.web.DownloadDriveFile(ctx, boundary, request.DocumentID, zone)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return &DownloadDriveFileResult{Content: response.Content.Body,
		TokenMetadata: publicMetadata(response.Token), Metadata: publicMetadata(response.Content)}, nil
}

// GetDriveNode retrieves the node selected by its provider identifier and optional sharing descriptor.
func (sdk *SDK) GetDriveNode(ctx context.Context, request GetDriveNodeRequest) (*GetDriveNodeResult, error) {
	const operation = "GetDriveNode"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	if request.NodeID == "" {
		return nil, newClientError(operation, Configuration, 0, nil, nil, errDriveNodeID)
	}

	boundary.Origin = request.Auth.DriveServiceURL
	boundary.DriveToken = request.Auth.DriveToken

	var share *drive.DriveShareID

	if request.ShareID != nil {
		value := drive.DriveShareID(*request.ShareID)
		share = &value
	}

	response, err := sdk.web.GetDriveNode(ctx, boundary, request.NodeID, share)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return &GetDriveNodeResult{Node: projectDriveNode(response.Data), Metadata: publicMetadata(response.Response)}, nil
}

// ListDriveLibraries fetches application-library records using caller-owned Drive authentication.
func (sdk *SDK) ListDriveLibraries(ctx context.Context,
	request ListDriveLibrariesRequest,
) (*ListDriveLibrariesResult, error) {
	const operation = "ListDriveLibraries"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	boundary.Origin = request.Auth.DriveServiceURL
	boundary.DriveToken = request.Auth.DriveToken

	response, err := sdk.web.ListDriveLibraries(ctx, boundary)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return &ListDriveLibrariesResult{Libraries: projectDriveNodes(response.Data.Items),
		AdditionalMetadata: copyAccountMetadata(response.Data.AdditionalProperties),
		Metadata:           publicMetadata(response.Response)}, nil
}

// CreateDriveFolder creates one named folder.
// The result acknowledges the request; background processing may continue.
func (sdk *SDK) CreateDriveFolder(ctx context.Context,
	request CreateDriveFolderRequest,
) (*CreateDriveFolderResult, error) {
	const operation = "CreateDriveFolder"

	boundary, err := driveRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.CreateDriveFolder(ctx, boundary, request.ParentID, request.Name)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDriveCreated(response), nil
}

// RenameDriveNode requests a new node name using its supplied version token.
// The result acknowledges the request; background processing may continue.
func (sdk *SDK) RenameDriveNode(ctx context.Context,
	request RenameDriveNodeRequest,
) (*RenameDriveNodeResult, error) {
	const operation = "RenameDriveNode"

	boundary, err := driveRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.RenameDriveNode(ctx, boundary, driveSelection(request.Node), request.Name)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDriveChanged(response), nil
}

// MoveDriveNodes requests movement of the selected nodes, including an empty selection.
// The result acknowledges the request; background processing may continue.
func (sdk *SDK) MoveDriveNodes(ctx context.Context,
	request MoveDriveNodesRequest,
) (*MoveDriveNodesResult, error) {
	const operation = "MoveDriveNodes"

	boundary, err := driveRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.MoveDriveNodes(ctx, boundary, driveSelections(request.Nodes), request.DestinationID)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDriveChanged(response), nil
}

// TrashDriveNode requests movement of a selected node into trash.
// The result acknowledges the request; background processing may continue.
func (sdk *SDK) TrashDriveNode(ctx context.Context,
	request TrashDriveNodeRequest,
) (*TrashDriveNodeResult, error) {
	const operation = "TrashDriveNode"

	boundary, err := driveRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.TrashDriveNode(ctx, boundary, driveSelection(request.Node))
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDriveChanged(response), nil
}

// RestoreDriveNode requests recovery of a selected trash node.
// The result acknowledges the request; background processing may continue.
func (sdk *SDK) RestoreDriveNode(ctx context.Context,
	request RestoreDriveNodeRequest,
) (*RestoreDriveNodeResult, error) {
	const operation = "RestoreDriveNode"

	boundary, err := driveRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.RestoreDriveNode(ctx, boundary, driveSelection(request.Node))
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDriveChanged(response), nil
}

// DeleteDriveNode requests ordinary deletion of a selected node.
// The result acknowledges the request; background processing may continue.
func (sdk *SDK) DeleteDriveNode(ctx context.Context,
	request DeleteDriveNodeRequest,
) (*DeleteDriveNodeResult, error) {
	const operation = "DeleteDriveNode"

	boundary, err := driveRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.DeleteDriveNode(ctx, boundary, driveSelection(request.Node))
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDriveChanged(response), nil
}

// PermanentlyDeleteDriveNode requests permanent deletion of a selected trash node.
// The result acknowledges the request; background processing may continue.
func (sdk *SDK) PermanentlyDeleteDriveNode(ctx context.Context,
	request PermanentlyDeleteDriveNodeRequest,
) (*PermanentlyDeleteDriveNodeResult, error) {
	const operation = "PermanentlyDeleteDriveNode"

	boundary, err := driveRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.web.PermanentlyDeleteDriveNode(ctx, boundary, driveSelection(request.Node))
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDriveChanged(response), nil
}

func accountRequestContext(auth AuthContext) (webtransport.RequestContext, error) {
	var boundary webtransport.RequestContext

	if auth.AccountID == "" || auth.ClientID == "" {
		return boundary, errAuthIdentifiers
	}

	params := new(accountapi.ListAccountDevicesParams)
	params.ClientId = auth.ClientID
	params.Dsid = auth.AccountID
	params.Accept = accountapi.ListAccountDevicesParamsAccept(accountapi.AcceptAsterisk)
	params.ClientBuildNumber = copyString(auth.ClientBuildNumber)
	params.ClientMasteringNumber = copyString(auth.ClientMasteringNumber)

	cookies, err := webtransport.NewCookieState(authCookies(auth.Cookies))
	if err != nil {
		return boundary, fmt.Errorf("account cookie context: %w", err)
	}

	boundary = webtransport.RequestContext{Origin: auth.AccountServiceURL, DriveToken: "", Params: *params,
		Headers: requestHeaders(auth.Headers), Cookies: cookies}

	return boundary, nil
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}

	copyValue := *value

	return &copyValue
}

func requestHeaders(headers []Header) http.Header {
	result := make(http.Header)
	for _, header := range headers {
		result.Add(header.Name, header.Value)
	}

	return result
}

func driveRequestContext(auth AuthContext) (webtransport.RequestContext, error) {
	boundary, err := accountRequestContext(auth)
	if err != nil {
		return boundary, fmt.Errorf("account cookie context: %w", err)
	}

	boundary.Origin = auth.DriveServiceURL
	boundary.DriveToken = auth.DriveToken

	return boundary, nil
}

func driveSelection(node DriveNodeSelector) webtransport.DriveSelection {
	return webtransport.DriveSelection{NodeID: node.NodeID, ETag: node.ETag}
}

func driveSelections(nodes []DriveNodeSelector) []webtransport.DriveSelection {
	result := make([]webtransport.DriveSelection, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, driveSelection(node))
	}

	return result
}

// WithClock supplies a concurrency-safe clock for default upload timestamps.
func WithClock(clock func() time.Time) Option {
	return func(config *configuration) error {
		if clock == nil {
			return errNilOption
		}

		config.clock = clock

		return nil
	}
}

// UploadDriveFile does not close the caller's reader or retry uncertain writes.
func (sdk *SDK) UploadDriveFile(ctx context.Context, request UploadDriveFileRequest) (*UploadDriveFileResult, error) {
	const operation = "UploadDriveFile"

	boundary, err := accountRequestContext(request.Auth)
	if err != nil {
		return nil, newClientError(operation, Configuration, 0, nil, nil, err)
	}

	boundary.Origin = request.Auth.DriveDocumentServiceURL

	zone := string(drive.ComAppleCloudDocs)
	if request.Zone != nil {
		zone = *request.Zone
	}

	response, err := sdk.web.UploadDriveFile(ctx, boundary, webtransport.DriveUploadInput{
		ParentID: request.ParentID, Filename: request.Filename, Content: request.Content, Zone: zone,
		ModificationTime: request.ModificationTime, CreationTime: request.CreationTime, Clock: sdk.clock,
	})
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDriveUpload(response), nil
}
