package webtransport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"regexp"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/driveapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

const uploadTokenMatchFields = 2

var (
	errUploadToken = errors.New("drive validation cookie has no upload token")
	errUploadFile  = errors.New("drive upload requires seekable content")
	uploadToken    = regexp.MustCompile(protocol.DriveUploadTokenCookieValuePattern)
)

func driveUploadToken(state *CookieState) (string, error) {
	if state != nil {
		for _, seed := range state.seeds {
			if seed.Cookie.Name == protocol.DriveUploadValidationCookieNameValue && seed.Cookie.Value != "" {
				match := uploadToken.FindStringSubmatch(seed.Cookie.Value)
				if len(match) == uploadTokenMatchFields {
					return match[1], nil
				}

				break
			}
		}
	}

	return "", errUploadToken
}

func uploadFileSize(content io.ReadSeeker) (int64, error) {
	if content == nil {
		return 0, errUploadFile
	}

	position, err := content.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("read upload cursor: %w", err)
	}

	size, err := content.Seek(0, io.SeekEnd)
	_, restoreErr := content.Seek(position, io.SeekStart)

	if err != nil || restoreErr != nil {
		return 0, fmt.Errorf("size upload file: %w", errors.Join(err, restoreErr))
	}

	return size, nil
}

func (client *Client) prepareDriveUpload(ctx context.Context, auth RequestContext,
	input DriveUploadInput,
) (*BytesResponse, drive.DriveUploadDestination, string, error) {
	size, err := uploadFileSize(input.Content)
	if err != nil {
		return nil, drive.DriveUploadDestination{}, "", failure(Configuration, err, nil, nil)
	}

	token, err := driveUploadToken(auth.Cookies)
	if err != nil {
		return nil, drive.DriveUploadDestination{}, "", failure(Configuration, err, nil, nil)
	}

	mediaType, _, _ := mime.ParseMediaType(mime.TypeByExtension(filepath.Ext(input.Filename)))
	payload := drive.DriveUploadRequest{Filename: input.Filename, Type: drive.FILE, ContentType: mediaType, Size: size}
	params := driveapi.DriveGetUploadDestinationParams{ClientId: auth.Params.ClientId, Dsid: auth.Params.Dsid,
		ClientBuildNumber: auth.Params.ClientBuildNumber, ClientMasteringNumber: auth.Params.ClientMasteringNumber,
		Token: token, Accept: nil, Cookie: nil, Origin: nil, Referer: nil, UserAgent: nil,
		AcceptEncoding: nil, Connection: nil}

	body, err := referenceJSON(payload)
	if err != nil {
		return nil, drive.DriveUploadDestination{}, token, failure(Configuration, err, nil, nil)
	}

	request, err := driveapi.NewDriveGetUploadDestinationRequestWithBody(auth.Origin, input.Zone, &params,
		protocol.DriveMediaPlainText, bytes.NewReader(body))
	if err != nil {
		return nil, drive.DriveUploadDestination{}, token, failure(Configuration, err, nil, nil)
	}

	response, err := client.readWithPolicy(ctx, plainTextUploadAuth(auth), request,
		"&"+queryPart(protocol.DriveGetUploadDestinationTokenName, token), successfulContent, true)
	if err != nil {
		return nil, drive.DriveUploadDestination{}, token, err
	}

	destination, err := decodeUploadDestination(response.Body)
	if err != nil {
		return nil, drive.DriveUploadDestination{}, token, responseFailure(Decode, err, response)
	}

	return response, destination, token, nil
}

func plainTextUploadAuth(auth RequestContext) RequestContext {
	auth.Headers = callerHeaders(auth.Headers)
	auth.Headers.Set(protocol.HTTPContentTypeName, protocol.DriveMediaPlainText)

	return auth
}
