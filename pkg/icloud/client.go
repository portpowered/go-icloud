package icloud

import (
	"context"
	"errors"
	"net/http"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/accounttransport"
)

// Client provides the currently implemented iCloud operations.
// Credentials belong to each request; a Client can serve multiple accounts.
type Client interface {
	// GetAccountDevices returns fresh devices and payment-method metadata.
	GetAccountDevices(ctx context.Context, request GetAccountDevicesRequest) (*GetAccountDevicesResult, error)
}

var (
	errTransportConfig = errors.New("HTTP transport must be nonnil and configured once")
	errNilOption       = errors.New("nil client option")
	errAuthIdentifiers = errors.New("account and client identifiers are required")
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

	if request.Auth.AccountID == "" || request.Auth.ClientID == "" {
		return nil, newClientError(operation, Configuration, 0, nil, nil,
			errAuthIdentifiers)
	}

	params := new(accountapi.ListAccountDevicesParams)
	params.ClientId = request.Auth.ClientID
	params.Dsid = request.Auth.AccountID
	params.Accept = accountapi.ListAccountDevicesParamsAccept(accountapi.AcceptAsterisk)
	params.ClientBuildNumber = copyString(request.Auth.ClientBuildNumber)
	params.ClientMasteringNumber = copyString(request.Auth.ClientMasteringNumber)

	boundary := accounttransport.RequestContext{
		Origin: request.Auth.AccountServiceURL, Params: *params, Headers: requestHeaders(request.Auth.Headers),
	}

	response, err := sdk.account.GetDevices(ctx, boundary)
	if err != nil {
		return nil, adaptFailure(operation, err)
	}

	return projectDevices(response), nil
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
