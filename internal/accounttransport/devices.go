// Package accounttransport owns schema-generated account requests and response bodies.
package accounttransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/account"
)

// Stage identifies which boundary failed without exposing provider or credential text.
type Stage string

var (
	errInvalidOrigin = errors.New("account service URL must be an HTTPS origin")
	errDeviceArray   = errors.New("account response has no device array")
	errProviderBody  = errors.New("provider reported an account failure")
)

// Failure stages separate configuration, network, provider and decoding failures.
const (
	Configuration Stage = "configuration"
	Transport     Stage = "transport"
	Provider      Stage = "provider"
	Decode        Stage = "decode"
)

// ResponseError preserves exact response evidence and the original failure cause.
type ResponseError struct {
	Stage   Stage
	Status  int
	Body    []byte
	Headers http.Header
	Cause   error
}

// Error is safe for display; raw response details remain separately inspectable.
func (failure *ResponseError) Error() string {
	return "account request failed: " + string(failure.Stage)
}

// Unwrap preserves cancellation and injected transport failure identity.
func (failure *ResponseError) Unwrap() error { return failure.Cause }

// RequestContext is caller-owned request state, never retained by Client.
type RequestContext struct {
	Origin  string
	Params  accountapi.ListAccountDevicesParams
	Headers http.Header
}

// DevicesResponse separates decoded wire data from HTTP response metadata.
type DevicesResponse struct {
	Data    account.AccountDevicesResponse
	Status  int
	Headers http.Header
}

// Client owns an HTTP client without a cookie jar or mutable account context.
type Client struct{ httpClient *http.Client }

// New constructs an account transport. Its injected transport must be safe for concurrent use.
func New(transport http.RoundTripper) *Client {
	client := new(http.Client)
	client.Transport = transport
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }

	return &Client{httpClient: client}
}

// GetDevices fetches a fresh device response using the supplied account context.
func (client *Client) GetDevices(ctx context.Context, auth RequestContext) (*DevicesResponse, error) {
	request, err := accountapi.NewListAccountDevicesRequest(auth.Origin, &auth.Params)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	response, err := client.read(ctx, auth, request, "")
	if err != nil {
		return nil, err
	}

	data, err := decodeDevices(response.Body)
	if err != nil {
		return nil, responseFailure(Decode, err, response)
	}

	return &DevicesResponse{Data: data, Status: response.Status, Headers: response.Headers}, nil
}

func failure(stage Stage, cause error, response *http.Response, body []byte) *ResponseError {
	result := &ResponseError{Stage: stage, Cause: cause, Body: body, Status: 0, Headers: nil}
	if response != nil {
		result.Status = response.StatusCode
		result.Headers = response.Header.Clone()
	}

	return result
}

func validateOrigin(origin string) error {
	parsed, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("parse account origin: %w", err)
	}

	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.ForceQuery {
		return errInvalidOrigin
	}

	return nil
}

func orderedAccountQuery(params accountapi.ListAccountDevicesParams) string {
	parts := make([]string, 0)
	if params.ClientBuildNumber != nil {
		parts = append(parts, queryPart(protocol.ClientBuildNumberName, *params.ClientBuildNumber))
	}

	if params.ClientMasteringNumber != nil {
		parts = append(parts, queryPart(protocol.ClientMasteringNumberName, *params.ClientMasteringNumber))
	}

	parts = append(parts, queryPart(protocol.ClientIDName, params.ClientId), queryPart(protocol.DSIDName, params.Dsid))

	return strings.Join(parts, "&")
}

func queryPart(name, value string) string {
	return url.QueryEscape(name) + "=" + url.QueryEscape(value)
}

func decodeDevices(body []byte) (account.AccountDevicesResponse, error) {
	var data account.AccountDevicesResponse

	var fields map[string]json.RawMessage

	err := json.Unmarshal(body, &fields)
	if err != nil {
		return data, fmt.Errorf("decode account envelope: %w", err)
	}

	devices, exists := fields[protocol.AccountDevicesResponseDevices]
	if !exists || len(devices) == 0 || devices[0] != '[' {
		return data, errDeviceArray
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("decode account devices: %w", err)
	}

	return data, nil
}

func providerError(fields map[string]json.RawMessage) bool {
	for _, name := range []string{protocol.AccountErrorErrorMessage, protocol.AccountErrorReason,
		protocol.AccountErrorErrorReason, protocol.AccountErrorError} {
		if providerTruth(fields[name]) {
			return true
		}
	}

	return false
}

func providerTruth(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}

	var value any

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()

	err := decoder.Decode(&value)
	if err != nil {
		return false
	}

	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed != ""
	case json.Number:
		return numberTruth(typed)
	case []any:
		return len(typed) != 0
	case map[string]any:
		return len(typed) != 0
	default:
		return false
	}
}

func numberTruth(value json.Number) bool {
	number, err := value.Float64()

	return number != 0 && (err == nil || math.IsInf(number, 0))
}
