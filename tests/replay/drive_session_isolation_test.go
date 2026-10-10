package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type driveSessionRouter struct{ alpha, beta *nodeReplayTraffic }

func (router driveSessionRouter) RoundTrip(request *http.Request) (*http.Response, error) {
	identity := request.URL.Query().Get(protocol.DSIDName)
	cookie := request.Header.Get(protocol.CookieName)

	var transport *nodeReplayTraffic

	switch {
	case identity == "account-alpha" || strings.Contains(cookie, replayLiteralSessionAlpha):
		transport = router.alpha
	case identity == "account-beta" || strings.Contains(cookie, replayLiteralSessionBeta):
		transport = router.beta
	default:
		return nil, errUnknownAccount
	}

	response, err := transport.RoundTrip(request)
	if err != nil {
		return nil, fmt.Errorf("session tenant routing: %w", err)
	}

	return response, nil
}

func tenantDriveNodeScenario(t *testing.T, tenant string) driveNodeScenario {
	t.Helper()

	data, err := os.ReadFile("fixtures/synthetic/http/drive-node-upload-refresh.json")
	if err != nil {
		t.Fatal(err)
	}

	replace := strings.NewReplacer(
		replayExpectedSyntheticClient, "client-"+tenant, "synthetic-account", replayExpectedAccount+tenant,
		"synthetic-root", "root-"+tenant, "synthetic-upload", "token-"+tenant,
		"session=uploaded", replayExpectedSessionUploaded+tenant)

	var scenario driveNodeScenario

	err = json.Unmarshal([]byte(replace.Replace(string(data))), &scenario)
	if err != nil {
		t.Fatal(err)
	}

	cookie := new(referenceCookie)
	cookie.Name, cookie.Value = nodeSessionCookieName, tenant
	cookie.Domain, cookie.Path = replayExpectedExampleInvalid, "/"

	scenario.Initial.Cookies = append(scenario.Initial.Cookies, *cookie)
	for index := range scenario.Exchanges {
		exchange := &scenario.Exchanges[index]
		exchange.Request.Body = tenantDriveNodeEntity(t, exchange.Request.Body, replace)
		exchange.Response.Body = tenantDriveNodeEntity(t, exchange.Response.Body, replace)
		fixTenantDriveLength(t, &exchange.Request)

		for headerIndex := range exchange.Request.Headers {
			if exchange.Request.Headers[headerIndex][0] == nodeCookieHeaderKey {
				value := replayExpectedSession + tenant
				if index == len(scenario.Exchanges)-1 {
					value = replayExpectedSessionUploaded + tenant
				}
				// Replace the fixture's post-upload session cookie before adding the seed value.
				parts := strings.Split(exchange.Request.Headers[headerIndex][1], "; session=")
				exchange.Request.Headers[headerIndex][1] = parts[0] + "; " + value
			}
		}
	}

	return scenario
}

func fixTenantDriveLength(t *testing.T, request *replay.Request) {
	t.Helper()

	if request.Body.Encoding != testBase64Encoding {
		return
	}

	var encoded string

	err := json.Unmarshal(request.Body.Value, &encoded)
	if err != nil {
		t.Fatal(err)
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	for index := range request.Headers {
		if request.Headers[index][0] == contentLengthHeader {
			request.Headers[index][1] = strconv.Itoa(len(data))
		}
	}
}

func tenantDriveNodeEntity(t *testing.T, entity replay.Entity, replace *strings.Replacer) replay.Entity {
	t.Helper()

	if entity.Encoding != testBase64Encoding {
		entity.Value = json.RawMessage(replace.Replace(string(entity.Value)))

		return entity
	}

	var encoded string

	err := json.Unmarshal(entity.Value, &encoded)
	if err != nil {
		t.Fatal(err)
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	entity.Value, err = json.Marshal(base64.StdEncoding.EncodeToString([]byte(replace.Replace(string(data)))))
	if err != nil {
		t.Fatal(err)
	}

	return entity
}

func TestDriveSessionsIsolateCachesTokensAndCookies(t *testing.T) {
	t.Parallel()
	alpha := tenantDriveNodeScenario(t, driveDownloadTenantAlpha)
	beta := tenantDriveNodeScenario(t, driveDownloadTenantBeta)

	alphaTransport, err := replay.NewHTTPTransport(alpha.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	betaTransport, err := replay.NewHTTPTransport(beta.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	alphaTraffic := &nodeReplayTraffic{base: alphaTransport, count: 0}
	betaTraffic := &nodeReplayTraffic{base: betaTransport, count: 0}

	client, err := icloud.New(icloud.WithHTTPTransport(driveSessionRouter{alpha: alphaTraffic, beta: betaTraffic}),
		icloud.WithClock(func() time.Time { return time.Unix(alpha.Entropy.Seconds, 0) }))
	if err != nil {
		t.Fatal(err)
	}

	runTenantDriveControls(t, client, alpha, beta, alphaTraffic, betaTraffic)
}

func runTenantDriveControls(t *testing.T, client *icloud.SDK, alpha, beta driveNodeScenario,
	alphaTraffic, betaTraffic *nodeReplayTraffic,
) {
	t.Helper()

	alphaTransport, betaTransport := alphaTraffic.base, betaTraffic.base

	scenarios := map[string]driveNodeScenario{driveDownloadTenantAlpha: alpha, driveDownloadTenantBeta: beta}
	for tenant, scenario := range scenarios {
		t.Run(tenant, func(t *testing.T) {
			t.Parallel()

			transport := alphaTransport
			traffic := alphaTraffic

			if tenant == driveDownloadTenantBeta {
				transport = betaTransport
				traffic = betaTraffic
			}

			auth := sdkAccountAuth(scenario.Initial)
			auth.DriveServiceURL, auth.DriveDocumentServiceURL = scenario.Initial.Origin, scenario.Initial.DocumentOrigin

			session, openErr := client.OpenDriveSession(t.Context(), icloud.OpenDriveSessionRequest{Auth: auth})
			if openErr != nil {
				t.Fatal(openErr)
			}

			t.Cleanup(func() {
				closeErr := session.Close()
				if closeErr != nil {
					t.Error(closeErr)
				}
			})

			runner := driveNodeRunner{scenario: scenario, session: session, root: nil, node: nil, traffic: traffic}

			result, callErr := runner.execute(t)
			if callErr != nil {
				t.Fatal(callErr)
			}

			checkSDKValue(t, result, scenario.Result)

			if session.Authentication().DriveToken != "token-"+tenant {
				t.Fatal("another account's upload token entered the session")
			}

			consumeErr := transport.AssertConsumed()
			if consumeErr != nil {
				t.Fatal(consumeErr)
			}
		})
	}
}
