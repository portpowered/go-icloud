//nolint:testpackage // GO-15: verifies the private signed locator constructor and its typed URL failure cause.
package webtransport

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestPhotoUploadLocatorPreservesEscapedSignedTarget(t *testing.T) {
	t.Parallel()

	const target = "https://upload.example.test/a%2Fb?sig=first&sig=second"

	content := strings.NewReader("abcde")

	_, err := content.Seek(2, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}

	request, err := photoUploadBytesRequest(target, content)
	if err != nil {
		t.Fatal(err)
	}

	if request.Method != http.MethodPost || request.URL.String() != target ||
		request.URL.EscapedPath() != "/a%2Fb" || request.ContentLength != 3 {
		t.Fatalf("upload target or framing changed: %s %s length=%d", request.Method, request.URL, request.ContentLength)
	}

	position, err := content.Seek(0, io.SeekCurrent)
	if err != nil || position != 2 {
		t.Fatalf("construction moved the caller cursor: position=%d error=%v", position, err)
	}

	body, err := io.ReadAll(request.Body)

	closeErr := request.Body.Close()
	if err != nil || closeErr != nil || string(body) != "cde" {
		t.Fatalf("upload body changed: body=%q read=%v close=%v", body, err, closeErr)
	}
}

func TestPhotoUploadLocatorRejectsUntrustedTarget(t *testing.T) {
	t.Parallel()

	for _, target := range []string{
		"http://upload.example.test/content",
		"https://user@upload.example.test/content",
		"https://upload.example.test/content#fragment",
		"https:///content",
		"/relative",
		"https://upload.example.test/%",
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			request, err := photoUploadBytesRequest(target, strings.NewReader("content"))

			var failure *ResponseError

			if request != nil || !errors.As(err, &failure) || failure.Stage != Configuration ||
				!errors.Is(err, errPhotoContentURL) {
				t.Fatalf("invalid upload target escaped configuration validation: request=%v error=%v", request, err)
			}

			if strings.HasSuffix(target, "/%") {
				var parseError *url.Error
				if !errors.As(err, &parseError) {
					t.Fatalf("URL parse cause was discarded: %v", err)
				}
			}
		})
	}
}
