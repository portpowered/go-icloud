package webtransport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/drivecontentapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

const uploadBoundaryBytes = 16

func uploadMultipart(input DriveUploadInput) ([]byte, string, error) {
	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	boundary := make([]byte, uploadBoundaryBytes)

	_, err := rand.Read(boundary)
	if err != nil {
		return nil, "", fmt.Errorf("upload boundary: %w", err)
	}

	err = writer.SetBoundary(hex.EncodeToString(boundary))
	if err != nil {
		return nil, "", fmt.Errorf("upload boundary: %w", err)
	}

	header := make(textproto.MIMEHeader)
	disposition := fmt.Sprintf(protocol.DriveUploadContentDispositionTemplate,
		uploadHeaderName(input.Filename), uploadHeaderName(filepath.Base(input.Filename)))
	header.Set(protocol.DriveUploadContentDispositionHeaderNameValue, disposition)

	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", fmt.Errorf("upload part: %w", err)
	}

	_, err = io.Copy(part, input.Content)
	if err != nil {
		return nil, "", fmt.Errorf("upload content: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, "", fmt.Errorf("finish multipart: %w", err)
	}

	return body.Bytes(), writer.FormDataContentType(), nil
}

func (client *Client) transferDriveUpload(ctx context.Context, auth RequestContext,
	input DriveUploadInput, contentURL string,
) (*BytesResponse, drive.DriveUploadReceipt, error) {
	var receipt drive.DriveUploadReceipt

	target, err := uploadTarget(contentURL)
	if err != nil {
		return nil, receipt, failure(Decode, errDriveShape, nil, nil)
	}

	body, contentType, err := uploadMultipart(input)
	if err != nil {
		return nil, receipt, failure(Transport, err, nil, nil)
	}

	request, err := drivecontentapi.NewDriveUploadContentRequestWithBody(target.Scheme+"://"+target.Host,
		target.EscapedPath(), nil, contentType, bytes.NewReader(body))
	if err != nil {
		return nil, receipt, failure(Configuration, err, nil, nil)
	}

	request.URL = target
	request.Header = callerHeaders(auth.Headers)
	request.Header.Set(protocol.HTTPContentTypeName, contentType)

	if request.Header.Get(protocol.AcceptName) == "" {
		request.Header.Set(protocol.AcceptName, anyMedia())
	}

	response, err := client.readPrepared(request.WithContext(ctx), successfulContent, auth.Cookies)
	if err != nil {
		return nil, receipt, err
	}

	receipt, err = decodeUploadReceipt(response.Body)
	if err != nil {
		return nil, receipt, responseFailure(Decode, err, response)
	}

	return response, receipt, nil
}

func uploadHeaderName(value string) string {
	return strings.NewReplacer("\r", "%0D", "\n", "%0A", "\"", "%22").Replace(value)
}

func uploadTarget(contentURL string) (*url.URL, error) {
	target, err := contentLocatorTarget(contentURL)
	if err != nil || target.Scheme != protocol.DriveHTTPSchemeValue || target.Host == "" ||
		target.User != nil || target.Fragment != "" {
		return nil, errDriveShape
	}

	return target, nil
}
