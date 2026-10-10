package webtransport

import (
	"bytes"
	"context"

	"github.com/portpowered/go-icloud/internal/protocol"
)

func (client *Client) postFindMy(ctx context.Context, auth RequestContext,
	payload any, accountQuery bool, build findMyRequestBuilder,
) (*BytesResponse, error) {
	err := validateOrigin(auth.Origin)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	body, err := referenceJSON(payload)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request, err := build(bytes.NewReader(body))
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	request = request.WithContext(ctx)

	request.Header = callerHeaders(auth.Headers)

	if request.Header == nil {
		request.Header = make(map[string][]string)
	}

	if request.Header.Get(protocol.HTTPContentTypeName) == "" {
		request.Header.Set(protocol.HTTPContentTypeName, jsonMedia())
	}

	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, anyMedia())
	}

	if accountQuery {
		request.URL.RawQuery = orderedAccountQuery(auth.Params)
	}

	return client.readPrepared(request, successfulContent, auth.Cookies)
}
