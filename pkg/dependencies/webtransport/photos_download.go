package webtransport

import (
	"context"
	"errors"
	"net/url"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/accountapi"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/photosapi"
)

var errPhotoContentURL = errors.New("invalid provider photo content URL")

// DownloadPhotoContent follows a URL selected from this operation's decoded photo resource.
// It preserves provider path/query bytes and sends no CloudKit query parameters.
func (client *Client) DownloadPhotoContent(ctx context.Context, auth RequestContext,
	contentURL string,
) (*BytesResponse, error) {
	target, err := url.Parse(contentURL)
	if err != nil || validateOrigin(target.Scheme+"://"+target.Host) != nil ||
		target.User != nil || target.Fragment != "" {
		return nil, failure(Decode, errPhotoContentURL, nil, nil)
	}

	request, err := photosapi.NewPhotosDownloadContentRequest(target.Scheme+"://"+target.Host, target.EscapedPath())
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	// Bind the exact escaped provider URL, including repeated signed query values.
	request.URL = target

	request.Header = CallerHeaders(auth.Headers)

	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, string(accountapi.AcceptAsterisk))
	}

	return client.readPrepared(request.WithContext(ctx), successfulContent, auth.Cookies)
}
