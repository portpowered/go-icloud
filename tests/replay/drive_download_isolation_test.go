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
	driveDownloadTenantAlpha = "alpha"
	driveDownloadTenantBeta  = "beta"
)

func tenantDriveDownloadReplay(t *testing.T, tenant string) (icloud.AuthContext, *replay.HTTPTransport) {
	t.Helper()

	scenario := readAccountScenario(t, "fixtures/synthetic/http/drive-download-data_token-binary.json")
	scenario.Initial.Params[protocol.ClientIDName] = "client-" + tenant
	scenario.Initial.Params[protocol.DSIDName] = "account-" + tenant
	scenario.Initial.Headers[protocol.CookieName] = "session=" + tenant

	for index := range scenario.Exchanges {
		exchange := &scenario.Exchanges[index]
		for queryIndex, pair := range exchange.Request.Query {
			if value, exists := scenario.Initial.Params[pair[0]]; exists {
				exchange.Request.Query[queryIndex][1] = value
			}
		}

		exchange.Request.Headers = append(exchange.Request.Headers,
			replay.Pair{strings.ToLower(protocol.CookieName), "session=" + tenant})

		stage := "token"
		if index == 1 {
			stage = "content"
		}

		exchange.Response.Headers = append(exchange.Response.Headers,
			replay.Pair{accountCookieUpdateHeader, driveDownloadCookieUpdate(stage, tenant)})
	}

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString([]byte("content-" + tenant)))
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

	alpha, alphaTransport := tenantDriveDownloadReplay(t, driveDownloadTenantAlpha)
	beta, betaTransport := tenantDriveDownloadReplay(t, driveDownloadTenantBeta)

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

				checkDriveDownloadTenant(t, result, tenant)
			}
		})
	}
}

func driveDownloadCookieUpdate(stage, tenant string) string {
	return "session=" + stage + "-" + tenant + "; Path=/; Secure; HttpOnly"
}

func checkDriveDownloadTenant(t *testing.T, result *icloud.DownloadDriveFileResult, tenant string) {
	t.Helper()

	if string(result.Content) != "content-"+tenant ||
		len(result.TokenMetadata.Headers) != 2 || len(result.Metadata.Headers) != 2 {
		t.Fatal("download crossed account contexts or lost intermediate updates")
	}

	if result.TokenMetadata.Headers[1].Name != accountCookieUpdateHeader ||
		result.Metadata.Headers[1].Name != accountCookieUpdateHeader ||
		result.TokenMetadata.Headers[1].Value != driveDownloadCookieUpdate("token", tenant) ||
		result.Metadata.Headers[1].Value != driveDownloadCookieUpdate("content", tenant) {
		t.Fatal("download metadata crossed stages or account contexts")
	}
}
