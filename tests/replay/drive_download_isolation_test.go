package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	driveDownloadTenantAlpha  = "alpha"
	driveDownloadTenantBeta   = "beta"
	driveDownloadCookieDomain = "; Domain=.example.invalid"
)

func tenantDriveDownloadReplay(t *testing.T, tenant string, structured bool,
) (icloud.AuthContext, *replay.HTTPTransport) {
	t.Helper()

	scenario := readAccountScenario(t, "fixtures/synthetic/http/drive-download-data_token-binary.json")
	scenario.Initial.Params[protocol.ClientIDName] = "client-" + tenant
	scenario.Initial.Params[protocol.DSIDName] = replayExpectedAccount + tenant

	initializeDriveDownloadCookies(&scenario.Initial, tenant, structured)

	for index := range scenario.Exchanges {
		exchange := &scenario.Exchanges[index]
		for queryIndex, pair := range exchange.Request.Query {
			if value, exists := scenario.Initial.Params[pair[0]]; exists {
				exchange.Request.Query[queryIndex][1] = value
			}
		}

		cookieValue := replayExpectedSession + tenant
		if structured && index == 1 {
			cookieValue = "session=token-" + tenant
		}

		exchange.Request.Headers = append(exchange.Request.Headers,
			replay.Pair{strings.ToLower(protocol.CookieName), cookieValue})

		stage := "token"
		if index == 1 {
			stage = "content"
		}

		exchange.Response.Headers = append(exchange.Response.Headers,
			replay.Pair{accountCookieUpdateHeader, driveDownloadCookieUpdate(stage, tenant)})
		if structured {
			exchange.Response.Headers[len(exchange.Response.Headers)-1][1] += driveDownloadCookieDomain
		}
	}

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString([]byte(replayExpectedContent + tenant)))
	if err != nil {
		t.Fatal(err)
	}

	scenario.Exchanges[1].Response.Body.Value = encoded
	sequence := make([]replay.Exchange, 0, len(scenario.Exchanges)*2)
	sequence = append(sequence, scenario.Exchanges...)
	sequence = append(sequence, scenario.Exchanges...)

	transport, err := replay.NewHTTPTransport(sequence)
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.DriveDocumentServiceURL = scenario.Initial.DocumentOrigin

	return auth, transport
}

func TestDriveDownloadSDKConcurrentIsolation(t *testing.T) {
	t.Parallel()

	for name, structured := range map[string]bool{"explicit-header": false, "structured-cookies": true} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkDriveDownloadIsolation(t, structured)
		})
	}
}

func checkDriveDownloadIsolation(t *testing.T, structured bool) {
	t.Helper()

	alpha, alphaTransport := tenantDriveDownloadReplay(t, driveDownloadTenantAlpha, structured)
	beta, betaTransport := tenantDriveDownloadReplay(t, driveDownloadTenantBeta, structured)

	client, err := icloud.New(icloud.WithHTTPTransport(accountRouter{alpha: alphaTransport, beta: betaTransport}))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		for _, transport := range []*replay.HTTPTransport{alphaTransport, betaTransport} {
			consumeErr := transport.AssertConsumed()
			if consumeErr != nil {
				t.Error(consumeErr)
			}
		}
	})

	for tenant, auth := range map[string]icloud.AuthContext{
		driveDownloadTenantAlpha: alpha, driveDownloadTenantBeta: beta,
	} {
		t.Run(tenant, func(t *testing.T) {
			t.Parallel()

			for range 2 {
				result, downloadErr := client.DownloadDriveFile(t.Context(), icloud.DownloadDriveFileRequest{
					Auth: auth, DocumentID: "synthetic-document", Zone: nil,
				})
				if downloadErr != nil {
					t.Fatal(downloadErr)
				}

				checkDriveDownloadTenant(t, result, tenant, structured)
			}
		})
	}
}

func driveDownloadCookieUpdate(stage, tenant string) string {
	return replayExpectedSession + stage + "-" + tenant + "; Path=/; Secure; HttpOnly"
}

func checkDriveDownloadTenant(t *testing.T, result *icloud.DownloadDriveFileResult, tenant string, structured bool) {
	t.Helper()

	if string(result.Content) != replayExpectedContent+tenant ||
		len(result.TokenMetadata.Headers) != 2 || len(result.Metadata.Headers) != 2 {
		t.Fatal("download crossed account contexts or lost intermediate updates")
	}

	tokenCookie := driveDownloadCookieUpdate("token", tenant)
	contentCookie := driveDownloadCookieUpdate("content", tenant)

	if structured {
		tokenCookie += driveDownloadCookieDomain
		contentCookie += driveDownloadCookieDomain
	}

	if result.TokenMetadata.Headers[1].Name != accountCookieUpdateHeader ||
		result.Metadata.Headers[1].Name != accountCookieUpdateHeader ||
		result.TokenMetadata.Headers[1].Value != tokenCookie ||
		result.Metadata.Headers[1].Value != contentCookie {
		t.Fatal("download metadata crossed stages or account contexts")
	}
}

func initializeDriveDownloadCookies(initial *accountInitial, tenant string, structured bool) {
	initial.Headers[protocol.CookieName] = replayExpectedSession + tenant

	if structured {
		delete(initial.Headers, protocol.CookieName)
		initial.Cookies = []referenceCookie{{Name: "session", Value: tenant, Domain: replayExpectedExampleInvalid,
			Path: "/", Secure: true, Expires: nil}}
	}
}
