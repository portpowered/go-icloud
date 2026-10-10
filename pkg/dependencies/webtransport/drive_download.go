package webtransport

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/accountapi"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/driveapi"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/drivecontentapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

// DriveDownloadResponse owns both exchange responses without retaining account credentials.
type DriveDownloadResponse struct {
	Token   *BytesResponse
	Content *BytesResponse
}

// DownloadDriveFile locates and retrieves exact bytes through the returned content URL.
func (client *Client) DownloadDriveFile(ctx context.Context, auth RequestContext,
	documentID, zone string,
) (*DriveDownloadResponse, error) {
	params := driveapi.DriveGetDownloadTokensParams{ClientId: auth.Params.ClientId, Dsid: auth.Params.Dsid, Token: nil,
		ClientBuildNumber: auth.Params.ClientBuildNumber, ClientMasteringNumber: auth.Params.ClientMasteringNumber,
		DocumentId: documentID, Accept: nil, Cookie: nil, Origin: nil, Referer: nil, UserAgent: nil,
		AcceptEncoding: nil, Connection: nil}

	request, err := driveapi.NewDriveGetDownloadTokensRequest(auth.Origin, zone, &params)
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	token, err := client.readDrive(ctx, auth, request,
		"&"+queryPart(protocol.DriveGetDownloadTokensDocumentIdName, documentID))
	if err != nil {
		return nil, err
	}

	var data drive.DriveDownloadTokens

	err = decodeDriveObject(token.Body, &data)
	if err != nil {
		return nil, responseFailure(Decode, err, token)
	}

	contentURL := downloadTokenURL(data)

	request, err = driveContentRequest(contentURL, auth)
	if err != nil {
		return nil, responseFailure(Decode, err, token)
	}

	content, err := client.readPrepared(request.WithContext(ctx), successfulContent, auth.Cookies)
	if err != nil {
		var responseError *ResponseError

		if errors.As(err, &responseError) {
			responseError.Prior = []*BytesResponse{token}
		}

		return nil, err
	}

	return &DriveDownloadResponse{Token: token, Content: content}, nil
}

func downloadTokenURL(tokens drive.DriveDownloadTokens) string {
	for _, token := range []*drive.DriveContentToken{tokens.DataToken, tokens.PackageToken} {
		if token != nil && token.Url != nil && *token.Url != "" {
			return *token.Url
		}
	}

	return ""
}

func driveContentRequest(contentURL string, auth RequestContext) (*http.Request, error) {
	target, err := url.Parse(contentURL)
	if err != nil || target.Scheme != protocol.DriveHTTPSchemeValue || target.Host == "" ||
		target.User != nil || target.Fragment != "" {
		return nil, errDriveShape
	}

	request, err := drivecontentapi.NewDriveDownloadContentRequest(target.Scheme+"://"+target.Host, target.EscapedPath())
	if err != nil {
		return nil, failure(Configuration, err, nil, nil)
	}

	// The generated template cannot preserve slash-separated and escaped provider paths.
	// Bind the complete target to this operation's decoded token, never to a caller URL.
	request.URL = target
	if request.URL.RawQuery != "" {
		request.URL.RawQuery += "&"
	}

	request.URL.RawQuery += orderedRequestQuery(auth)

	request.Header = CallerHeaders(auth.Headers)
	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, string(accountapi.AcceptAsterisk))
	}

	return request, nil
}
