package accounttransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/internal/protocol"
)

// BytesResponse owns read/closed content and copied response headers.
type BytesResponse struct {
	Body    []byte
	Status  int
	Headers http.Header
}

func (client *Client) read(ctx context.Context, auth RequestContext,
	request *http.Request, suffix string,
) (*BytesResponse, error) {
	err := validateOrigin(auth.Origin)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request = request.WithContext(ctx)

	request.Header = auth.Headers.Clone()
	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, string(accountapi.AcceptAsterisk))
	}

	request.URL.RawQuery = orderedAccountQuery(auth.Params) + suffix

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, failure(Transport, err, nil, nil)
	}

	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()

	err = errors.Join(readErr, closeErr)
	if err != nil {
		return nil, failure(Transport, err, response, body)
	}

	if response.StatusCode != http.StatusOK {
		return nil, failure(Provider, nil, response, body)
	}

	result := &BytesResponse{Body: body, Status: response.StatusCode, Headers: response.Header.Clone()}
	if responseProviderError(result) {
		return nil, responseFailure(Provider, errProviderBody, result)
	}

	return result, nil
}

func responseProviderError(response *BytesResponse) bool {
	mediaType, _, _ := strings.Cut(response.Headers.Get(protocol.HTTPContentTypeName), ";")
	if mediaType != protocol.MediaApplicationJson && mediaType != protocol.MediaTextJson {
		return false
	}

	var fields map[string]json.RawMessage

	err := json.Unmarshal(response.Body, &fields)

	return err == nil && providerError(fields)
}

func responseFailure(stage Stage, cause error, response *BytesResponse) *ResponseError {
	return &ResponseError{Stage: stage, Cause: cause, Body: response.Body,
		Status: response.Status, Headers: response.Headers}
}
