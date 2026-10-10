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

func tenantDriveRenameReplay(t *testing.T, tenant string) (icloud.AuthContext, *replay.HTTPTransport) {
	t.Helper()

	scenario := readAccountScenario(t, "fixtures/synthetic/http/drive-rename-success.json")
	scenario.Initial.Params[protocol.ClientIDName] = "client-" + tenant
	scenario.Initial.Params[protocol.DSIDName] = replayExpectedAccount + tenant
	scenario.Initial.Headers[protocol.CookieName] = replayExpectedSession + tenant

	exchange := scenario.Exchanges[0]
	for index, pair := range exchange.Request.Query {
		exchange.Request.Query[index][1] = scenario.Initial.Params[pair[0]]
	}

	exchange.Request.Headers = append(exchange.Request.Headers, replay.Pair{"cookie", replayExpectedSession + tenant})
	exchange.Response.Headers = append(exchange.Response.Headers,
		replay.Pair{accountCookieUpdateHeader, replayLiteralSessionUpdated + tenant + replayLiteralPathSecureHTTPOnly})

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString([]byte(
		`{"items":[{"name":"node-` + tenant + `"}]}`)))
	if err != nil {
		t.Fatal(err)
	}

	exchange.Response.Body.Value = encoded

	transport, err := replay.NewHTTPTransport([]replay.Exchange{exchange, exchange})
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.AccountServiceURL = ""
	auth.DriveServiceURL = scenario.Initial.Origin

	return auth, transport
}

func TestDriveMutationSDKConcurrentIsolation(t *testing.T) {
	t.Parallel()

	alpha, alphaTransport := tenantDriveRenameReplay(t, "alpha")
	beta, betaTransport := tenantDriveRenameReplay(t, "beta")

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

	for tenant, auth := range map[string]icloud.AuthContext{"alpha": alpha, "beta": beta} {
		t.Run(tenant, func(t *testing.T) {
			t.Parallel()

			for range 2 {
				checkDriveTenantRename(t, client, tenant, auth)
			}
		})
	}
}

func checkDriveTenantRename(t *testing.T, client icloud.Client, tenant string, auth icloud.AuthContext) {
	t.Helper()

	result, err := client.RenameDriveNode(t.Context(), icloud.RenameDriveNodeRequest{
		Auth: auth, Node: icloud.DriveNodeSelector{NodeID: "FILE::synthetic::one", ETag: "synthetic-etag"},
		Name: "renamed.txt",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Items == nil || len(*result.Items) != 1 || (*result.Items)[0].Name == nil ||
		*(*result.Items)[0].Name != "node-"+tenant {
		t.Fatal("Drive mutation result crossed account contexts")
	}

	if len(result.Metadata.Headers) != 2 ||
		!strings.Contains(result.Metadata.Headers[1].Value, replayLiteralUpdated+tenant) {
		t.Fatal("Drive session update headers were lost")
	}
}
