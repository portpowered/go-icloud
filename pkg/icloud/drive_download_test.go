package icloud_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const testDriveContentURL = "https://content.example.invalid/a%2Fb/c%20d?tag=a&tag=b&clientId=provider"

func driveDownloadRequest() icloud.DownloadDriveFileRequest {
	auth := driveAuth()
	auth.DriveDocumentServiceURL = "https://documents.example.invalid"

	return icloud.DownloadDriveFileRequest{Auth: auth, DocumentID: "synthetic-document", Zone: nil}
}

func TestDriveDownloadPreservesIssuedURLAndBinaryJSON(t *testing.T) {
	t.Parallel()

	for name, tokens := range map[string]string{
		"data": `{"data_token":{"url":"` + testDriveContentURL +
			`"},"package_token":{"url":"https://wrong.example.invalid/file"}}`,
		"package": `{"data_token":{"url":""},"package_token":{"url":"` + testDriveContentURL + `"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := driveDownloadRequest()
			zone := "custom zone"
			request.Zone = &zone
			content := []byte{0, 255, '\n'}
			calls := 0

			client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(out *http.Request) (*http.Response, error) {
				calls++

				body := tokens

				if calls == 1 {
					if out.URL.EscapedPath() != "/ws/custom%20zone/download/by_id" {
						t.Fatal("document zone was not encoded")
					}
				} else {
					if out.URL.EscapedPath() != "/a%2Fb/c%20d" ||
						out.URL.RawQuery != "tag=a&tag=b&clientId=provider&clientId=synthetic-client&dsid=synthetic-account" {
						t.Fatal("provider path or ordered repeated query changed")
					}

					body = string(content)
				}

				response := new(http.Response)
				response.StatusCode = http.StatusOK

				response.Header = make(http.Header)
				response.Header.Set("Content-Type", "application/json")
				response.Body = io.NopCloser(strings.NewReader(body))

				return response, nil
			})))
			if err != nil {
				t.Fatal(err)
			}

			result, err := client.DownloadDriveFile(t.Context(), request)
			if err != nil || calls != 2 || !bytes.Equal(result.Content, content) {
				t.Fatalf("download bytes or token preference changed: %v", err)
			}
		})
	}
}

func TestDriveDownloadRejectsProviderTokenShapes(t *testing.T) {
	t.Parallel()

	for _, body := range []string{testJSONNull, `[]`, invalidProviderJSON, `{}`,
		`{"data_token":false}`, `{"data_token":{"url":"http://content.example.invalid/file"}}`,
		`{"data_token":{"url":"https://user@content.example.invalid/file"}}`,
		`{"data_token":{"url":"https://content.example.invalid/file#fragment"}}`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			client := sdkForResponse(t, http.StatusOK, body)
			result, err := client.DownloadDriveFile(t.Context(), driveDownloadRequest())

			var failure *icloud.ClientError

			if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse ||
				string(failure.ResponseBody()) != body || failure.StatusCode() != http.StatusOK {
				t.Fatalf("token rejection lost evidence: %v", err)
			}
		})
	}
}

func TestDriveDownloadOwnsBodiesOnEitherStageFailure(t *testing.T) {
	t.Parallel()

	for _, failingStage := range []int{1, 2} {
		t.Run(strconv.Itoa(failingStage), func(t *testing.T) {
			t.Parallel()

			calls := 0
			body := new(failingAccountBody)

			client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
				calls++
				response := new(http.Response)
				response.StatusCode = http.StatusOK
				response.Header = make(http.Header)
				response.Header.Set("Set-Cookie", "synthetic-update=value")

				response.Body = body
				if calls != failingStage {
					response.Body = io.NopCloser(strings.NewReader(`{"data_token":{"url":"` + testDriveContentURL + `"}}`))
				}

				return response, nil
			})))
			if err != nil {
				t.Fatal(err)
			}

			result, err := client.DownloadDriveFile(t.Context(), driveDownloadRequest())

			var failure *icloud.ClientError

			if result != nil || !body.closed || calls != failingStage || !errors.As(err, &failure) ||
				failure.Kind() != icloud.Transport {
				t.Fatal("download suppressed cleanup or failure causes")
			}

			assertDownloadCleanupCauses(t, err)
			assertDownloadPriorMetadata(t, failure, failingStage-1)
		})
	}
}

func assertDownloadCleanupCauses(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, errAccountRead) || !errors.Is(err, errAccountClose) {
		t.Fatal("download suppressed cleanup causes")
	}
}

func assertDownloadPriorMetadata(t *testing.T, failure *icloud.ClientError, expected int) {
	t.Helper()

	prior := failure.PriorResponses()
	if len(prior) != expected {
		t.Fatal("download lost preceding authentication metadata")
	}

	if len(prior) == 0 {
		return
	}

	prior[0].Headers[0].Value = "changed"

	if failure.PriorResponses()[0].Headers[0].Value != "synthetic-update=value" {
		t.Fatal("failure metadata aliases caller memory")
	}
}

func TestDriveDownloadAcceptsSuccessfulContentStatuses(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusNoContent, http.StatusPartialContent} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			t.Parallel()

			calls := 0

			client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
				calls++
				response := new(http.Response)
				response.Header = make(http.Header)
				response.StatusCode = http.StatusOK
				response.Body = io.NopCloser(strings.NewReader(`{"data_token":{"url":"` + testDriveContentURL + `"}}`))

				if calls == 2 {
					response.StatusCode = status

					content := "partial content"
					if status == http.StatusNoContent {
						content = ""
					}

					response.Body = io.NopCloser(strings.NewReader(content))
				}

				return response, nil
			})))
			if err != nil {
				t.Fatal(err)
			}

			result, err := client.DownloadDriveFile(t.Context(), driveDownloadRequest())
			if err != nil || calls != 2 || result.Metadata.StatusCode != status {
				t.Fatalf("successful content status was treated as a provider failure: %v", err)
			}
		})
	}
}
