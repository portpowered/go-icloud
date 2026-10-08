package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"github.com/portpowered/go-icloud/internal/protocol"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const persistedContentCookie = "session=content; Secure; HttpOnly"

func cookiePersistenceSequence(t *testing.T) (icloud.AuthContext, *replay.HTTPTransport) {
	t.Helper()

	first := readAccountScenario(t, "fixtures/synthetic/http/drive-download-data_token-binary.json")
	second := readAccountScenario(t, "fixtures/synthetic/http/drive-download-data_token-binary.json")
	third := readAccountScenario(t, "fixtures/synthetic/http/drive-download-data_token-binary.json")

	first.Exchanges[1].Response.Headers = append(first.Exchanges[1].Response.Headers,
		replay.Pair{accountCookieUpdateHeader, persistedContentCookie})

	second.Exchanges[1].Request.Headers = append(second.Exchanges[1].Request.Headers,
		replay.Pair{strings.ToLower(protocol.CookieName), "session=content"})
	third.Exchanges[1].Request.Origin = "https://sub.content.example.invalid"

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString([]byte(
		`{"data_token":{"url":"https://sub.content.example.invalid/synthetic-file"}}`)))
	if err != nil {
		t.Fatal(err)
	}

	third.Exchanges[0].Response.Body.Value = encoded

	sequence := append([]replay.Exchange(nil), first.Exchanges...)
	sequence = append(sequence, second.Exchanges...)
	sequence = append(sequence, third.Exchanges...)

	transport, err := replay.NewHTTPTransport(sequence)
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(first.Initial)
	auth.DriveDocumentServiceURL = first.Initial.DocumentOrigin

	return auth, transport
}

func TestDriveDownloadCallerPersistsHostOnlyCookie(t *testing.T) {
	t.Parallel()

	auth, transport := cookiePersistenceSequence(t)

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	request := icloud.DownloadDriveFileRequest{Auth: auth, DocumentID: "synthetic-document", Zone: nil}

	first, err := client.DownloadDriveFile(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}

	if first.TokenMetadata.CookieScopeURL != "https://documents.example.invalid/ws/com.apple.CloudDocs/download/by_id" ||
		first.Metadata.CookieScopeURL != "https://content.example.invalid/synthetic-file" {
		t.Fatal("response cookie provenance lost its issuer or disclosed query credentials")
	}

	request.Auth.Cookies = []icloud.AuthCookie{persistedHostCookie(t, first.Metadata)}
	for range 2 {
		result, downloadErr := client.DownloadDriveFile(t.Context(), request)
		if downloadErr != nil || result == nil {
			t.Fatalf("persisted cookie download: %v", downloadErr)
		}
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func persistedHostCookie(t *testing.T, metadata icloud.ResponseMetadata) icloud.AuthCookie {
	t.Helper()

	target, err := url.Parse(metadata.CookieScopeURL)
	if err != nil {
		t.Fatal(err)
	}

	response := new(http.Response)
	response.Header = make(http.Header)

	for _, header := range metadata.Headers {
		response.Header.Add(header.Name, header.Value)
	}

	cookies := response.Cookies()
	if len(cookies) != 1 || cookies[0].Domain != "" || cookies[0].Path != "" {
		t.Fatal("expected a host-only cookie with default path")
	}

	return icloud.AuthCookie{Name: cookies[0].Name, Value: cookies[0].Value, Domain: target.Hostname(),
		HostOnly: true, Path: "/", Secure: cookies[0].Secure, HTTPOnly: cookies[0].HttpOnly,
		Expires: nil, MaxAge: 0, SameSite: nil}
}
