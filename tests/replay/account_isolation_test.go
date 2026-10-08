package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const accountCookieUpdateHeader = "Set-Cookie"

var errUnknownAccount = errors.New("unexpected synthetic account cookie")

type accountRouter struct{ alpha, beta *replay.HTTPTransport }

func (router accountRouter) RoundTrip(request *http.Request) (*http.Response, error) {
	var transport *replay.HTTPTransport

	switch request.Header.Get(protocol.CookieName) {
	case "session=alpha", "session=token-alpha":
		transport = router.alpha
	case "session=beta", "session=token-beta":
		transport = router.beta
	default:
		return nil, errUnknownAccount
	}

	response, err := transport.RoundTrip(request)
	if err != nil {
		return nil, fmt.Errorf("synthetic account routing: %w", err)
	}

	return response, nil
}

func tenantDeviceReplay(t *testing.T, tenant string) (icloud.AuthContext, *replay.HTTPTransport) {
	t.Helper()

	scenario := readAccountScenario(t, "fixtures/synthetic/http/account-devices-one.json")
	scenario.Initial.Params[protocol.ClientIDName] = "client-" + tenant
	scenario.Initial.Params[protocol.DSIDName] = "account-" + tenant
	scenario.Initial.Headers[protocol.CookieName] = "session=" + tenant
	scenario.Initial.Params[protocol.ClientBuildNumberName] = "synthetic-build"
	scenario.Initial.Params[protocol.ClientMasteringNumberName] = "synthetic-mastering"

	exchange := scenario.Exchanges[0]
	for index, pair := range exchange.Request.Query {
		exchange.Request.Query[index][1] = scenario.Initial.Params[pair[0]]
	}

	exchange.Request.Query = append([]replay.Pair{
		{protocol.ClientBuildNumberName, "synthetic-build"},
		{protocol.ClientMasteringNumberName, "synthetic-mastering"},
	}, exchange.Request.Query...)

	exchange.Request.Headers = append(exchange.Request.Headers, replay.Pair{"cookie", "session=" + tenant})
	exchange.Response.Headers = append(exchange.Response.Headers,
		replay.Pair{accountCookieUpdateHeader, "session=updated-" + tenant + "; Path=/; Secure; HttpOnly"})

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString([]byte(
		`{"devices":[{"name":"device-` + tenant + `","modelDisplayName":"Synthetic Model"}]}`)))
	if err != nil {
		t.Fatal(err)
	}

	exchange.Response.Body.Value = encoded

	transport, err := replay.NewHTTPTransport([]replay.Exchange{exchange, exchange})
	if err != nil {
		t.Fatal(err)
	}

	return sdkAccountAuth(scenario.Initial), transport
}

func TestAccountDevicesSDKConcurrentIsolation(t *testing.T) {
	t.Parallel()

	alpha, alphaTransport := tenantDeviceReplay(t, "alpha")
	beta, betaTransport := tenantDeviceReplay(t, "beta")

	client, err := icloud.New(icloud.WithHTTPTransport(accountRouter{alpha: alphaTransport, beta: betaTransport}))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		for _, transport := range []*replay.HTTPTransport{alphaTransport, betaTransport} {
			err = transport.AssertConsumed()
			if err != nil {
				t.Error(err)
			}
		}
	})

	for tenant, auth := range map[string]icloud.AuthContext{"alpha": alpha, "beta": beta} {
		t.Run(tenant, func(t *testing.T) {
			t.Parallel()

			checkSDKAccountIsolation(t, client, tenant, auth)
		})
	}
}

func checkSDKAccountIsolation(t *testing.T, client icloud.Client, tenant string, auth icloud.AuthContext) {
	t.Helper()

	original, encodeErr := json.Marshal(auth)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}

	for range 2 {
		response, requestErr := client.GetAccountDevices(t.Context(), icloud.GetAccountDevicesRequest{Auth: auth})
		checkSDKAccountDevice(t, response, tenant, requestErr)
	}

	after, encodeErr := json.Marshal(auth)
	if encodeErr != nil || string(original) != string(after) {
		t.Fatal("request auth was mutated")
	}
}

func checkSDKAccountDevice(t *testing.T, response *icloud.GetAccountDevicesResult, tenant string, requestErr error) {
	t.Helper()

	if requestErr != nil || response == nil || len(response.Devices) != 1 ||
		response.Devices[0].Name == nil || *response.Devices[0].Name != "device-"+tenant {
		t.Fatalf("account device data crossed contexts: %v", requestErr)
	}

	if len(response.Metadata.Headers) != 2 ||
		!strings.Contains(response.Metadata.Headers[1].Value, "updated-"+tenant) {
		t.Fatal("caller-owned session update headers were lost")
	}
}
