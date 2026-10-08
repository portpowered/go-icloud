package icloud

import (
	"context"
	"errors"
	"net/http"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/accounttransport"
	"github.com/portpowered/go-icloud/internal/protocol"
)

// Client provides the currently implemented iCloud operations.
// Credentials belong to each request; a Client can serve multiple accounts.
type Client interface {
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
)

type configuration struct {
	transport  http.RoundTripper
	configured bool
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
type SDK struct{ account *accounttransport.Client }

// New creates a reusable stateless client. The caller supplies request deadlines.
// The default is http.DefaultTransport; automatic redirects are disabled.
func New(options ...Option) (*SDK, error) {
	config := configuration{transport: http.DefaultTransport, configured: false}

	for _, option := range options {
		if option == nil {
			return nil, newClientError("New", Configuration, 0, nil, nil, errNilOption)
		}

		err := option(&config)
		if err != nil {
			return nil, newClientError("New", Configuration, 0, nil, nil, err)
		}
	}

	return &SDK{account: accounttransport.New(config.transport)}, nil
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

	response, err := sdk.account.GetDevices(ctx, boundary)
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

	response, err := sdk.account.GetFamily(ctx, boundary)
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

	response, err := sdk.account.GetMemberPhoto(ctx, boundary, request.MemberID)
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

	response, err := sdk.account.GetStorage(ctx, boundary)
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

	response, err := sdk.account.GetPlanSummary(ctx, boundary)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return &GetAccountPlanSummaryResult{Summary: response.Body, Metadata: publicMetadata(response)}, nil
}

func accountRequestContext(auth AuthContext) (accounttransport.RequestContext, error) {
	var boundary accounttransport.RequestContext

	if auth.AccountID == "" || auth.ClientID == "" {
		return boundary, errAuthIdentifiers
	}

	params := new(accountapi.ListAccountDevicesParams)
	params.ClientId = auth.ClientID
	params.Dsid = auth.AccountID
	params.Accept = accountapi.ListAccountDevicesParamsAccept(accountapi.AcceptAsterisk)
	params.ClientBuildNumber = copyString(auth.ClientBuildNumber)
	params.ClientMasteringNumber = copyString(auth.ClientMasteringNumber)

	boundary = accounttransport.RequestContext{Origin: auth.AccountServiceURL, Params: *params,
		Headers: requestHeaders(auth.Headers)}

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
