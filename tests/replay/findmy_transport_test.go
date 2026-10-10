package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
	"github.com/portpowered/go-icloud/tests/replay"
)

type findMyTransportScenario struct {
	//nolint:tagliatelle // LIB-05: portable scenario field spelling.
	Initial   findMyTransportInitial `json:"initial_state"`
	Exchanges []replay.Exchange      `json:"exchanges"`
	Inputs    []json.RawMessage      `json:"inputs"`
	//nolint:tagliatelle // LIB-05: portable scenario field spelling.
	DeviceID string `json:"device_id"`
	//nolint:tagliatelle // LIB-05: portable provider-error response evidence.
	ErrorContext *findMyTransportErrorContext `json:"error_context"`
}

type findMyTransportErrorContext struct {
	Response *replay.Response `json:"response"`
}

type findMyTransportInitial struct {
	accountInitial

	//nolint:tagliatelle // LIB-05: portable scenario field spelling.
	TokenOrigin string `json:"token_origin"`
	//nolint:tagliatelle // LIB-05: portable scenario field spelling.
	Family bool `json:"with_family"`
	//nolint:tagliatelle // LIB-05: portable scenario field spelling.
	SessionData map[string]string `json:"session_data"`
}

type findMyTransportState struct {
	auth         webtransport.RequestContext
	server       findmy.FindMyRefreshContext
	eraseToken   string
	family       bool
	tokenOrigin  string
	sessionToken *string
}

func TestFindMyTransportPortableExchanges(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(replayLiteralFixturesSyntheticHTTPFindmyJSON)
	if err != nil {
		t.Fatal(err)
	}

	pairs := 0

	for _, path := range paths {
		data, readErr := os.ReadFile(filepath.Clean(path))
		if readErr != nil {
			t.Fatal(readErr)
		}

		var scenario findMyTransportScenario

		decodeErr := json.Unmarshal(data, &scenario)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		pairs += len(scenario.Exchanges)

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runFindMyTransport(t, scenario, scenario.Initial)
		})
	}

	if len(paths) != 71 || pairs != 140 {
		t.Fatal("Find My transport inventory changed")
	}
}

func runFindMyTransport(t *testing.T, scenario findMyTransportScenario, initial findMyTransportInitial) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client := webtransport.New(transport)
	state := findMyInitialTransportState(t, initial)

	for _, exchange := range scenario.Exchanges {
		response, callErr := callFindMyTransport(t, client, &state, scenario, exchange.Request.Path)
		providerFailure := scenario.ErrorContext != nil &&
			reflect.DeepEqual(exchange.Response, scenario.ErrorContext.Response)
		assertFindMyTransportResponse(t, exchange, response, callErr, providerFailure)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func findMyInitialTransportState(t *testing.T, initial findMyTransportInitial) findMyTransportState {
	t.Helper()

	params := new(webtransport.RequestContext)
	params.Params.ClientId = initial.Params[protocol.FindMyClientIDName]
	params.Params.Dsid = initial.Params[protocol.FindMyDSIDName]

	params.Headers = make(http.Header)
	for name, value := range initial.Headers {
		params.Headers.Add(name, value)
	}

	params.Origin = initial.Origin
	// This caller-owned stream jar binds received cookies across transport-only calls.
	// The public SDK separately verifies explicit session credential persistence.
	cookies, err := webtransport.NewCookieState(nil)
	if err != nil {
		t.Fatal(err)
	}

	params.Cookies = cookies

	target, err := url.Parse(initial.TokenOrigin)
	if err != nil {
		t.Fatal(err)
	}

	var token *string
	if value, exists := initial.SessionData[replayExpectedSessionStateField]; exists {
		token = &value
	}

	return findMyTransportState{auth: *params,
		server: nil, eraseToken: "",
		family: initial.Family, tokenOrigin: target.Scheme + "://" + target.Host, sessionToken: token}
}

func callFindMyTransport(t *testing.T, client *webtransport.Client, state *findMyTransportState,
	scenario findMyTransportScenario, path string,
) (*webtransport.BytesResponse, error) {
	t.Helper()

	switch path {
	case protocol.FindMyInitializePath, protocol.FindMyRefreshDevicesPath:
		return callFindMyTransportRead(t, client, state, scenario, path)
	case protocol.FindMyObtainEraseTokenPath:
		return callFindMyTransportToken(t, client, state)
	default:
		return callFindMyTransportCommand(t, client, state, scenario, path)
	}
}

func callFindMyTransportCommand(t *testing.T, client *webtransport.Client, state *findMyTransportState,
	scenario findMyTransportScenario, path string,
) (*webtransport.BytesResponse, error) {
	t.Helper()

	var (
		response *webtransport.BytesResponse
		err      error
	)

	switch path {
	case protocol.FindMyPlaySoundPath:
		response, err = client.PlayFindMySound(t.Context(), state.auth, scenario.DeviceID,
			findMyStringInput(t, scenario, 0))
	case protocol.FindMySendMessagePath:
		payload := findmy.FindMyMessageRequest{Device: scenario.DeviceID,
			Subject: findMyStringInput(t, scenario, 0), UserText: findmy.FindMyMessageRequestUserTextTrue,
			Text: findMyStringInput(t, scenario, 1), Sound: findMyBoolInput(t, scenario, 2),
			Vibrate: findMyBoolInput(t, scenario, 3), Strobe: findMyBoolInput(t, scenario, 4)}

		response, err = client.SendFindMyMessage(t.Context(), state.auth, payload)
	case protocol.FindMyLostDevicePath:
		payload := findmy.FindMyLostRequest{Text: findMyStringInput(t, scenario, 1),
			UserText: findmy.FindMyLostRequestUserTextTrue, OwnerNbr: findMyStringInput(t, scenario, 0),
			LostModeEnabled: findmy.FindMyLostRequestLostModeEnabledTrue,
			TrackingEnabled: findmy.FindMyLostRequestTrackingEnabledTrue, Device: scenario.DeviceID,
			Passcode: findMyStringInput(t, scenario, 2)}

		response, err = client.MarkFindMyLost(t.Context(), state.auth, payload)
	case protocol.FindMyEraseDevicePath:
		payload := findmy.FindMyEraseRequest{AuthToken: state.eraseToken,
			Text: findMyStringInput(t, scenario, 0), Device: scenario.DeviceID,
			Passcode: findMyStringInput(t, scenario, 1)}

		response, err = client.EraseFindMyDevice(t.Context(), state.auth, payload)
	default:
		t.Fatal("unregistered Find My route", path)

		return nil, errFindMyTestRoute
	}

	if err != nil {
		return nil, fmt.Errorf("Find My command: %w", err)
	}

	return response, nil
}

var errFindMyTestRoute = errors.New("unregistered Find My test route")

func callFindMyTransportToken(t *testing.T, client *webtransport.Client,
	state *findMyTransportState,
) (*webtransport.BytesResponse, error) {
	t.Helper()

	auth := state.auth
	auth.Origin = state.tokenOrigin

	response, err := client.ObtainFindMyEraseToken(t.Context(), auth, state.sessionToken)
	if err != nil {
		return nil, fmt.Errorf("Find My token lookup: %w", err)
	}

	assertFindMyDecoded(t, response.Data, response.Response.Body)

	if response.Data.Tokens != nil && response.Data.Tokens.MmeFMIPWebEraseDeviceToken != nil {
		state.eraseToken = *response.Data.Tokens.MmeFMIPWebEraseDeviceToken
	}

	return response.Response, nil
}

func callFindMyTransportRead(t *testing.T, client *webtransport.Client, state *findMyTransportState,
	scenario findMyTransportScenario, path string,
) (*webtransport.BytesResponse, error) {
	t.Helper()

	var (
		response *webtransport.FindMyDevicesResponse
		err      error
	)

	if path == protocol.FindMyInitializePath {
		response, err = client.InitializeFindMy(t.Context(), state.auth, state.family)
	} else {
		locate := true

		if len(scenario.Inputs) != 0 {
			var input struct {
				Locate bool `json:"locate"`
			}

			decodeErr := json.Unmarshal(scenario.Inputs[0], &input)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}

			locate = input.Locate
		}

		response, err = client.RefreshFindMy(t.Context(), state.auth, state.server, state.family, locate)
	}

	if err != nil {
		return nil, fmt.Errorf("Find My discovery: %w", err)
	}

	assertFindMyDecoded(t, response.Data, response.Response.Body)

	state.server = response.Context

	return response.Response, nil
}

func findMyStringInput(t *testing.T, scenario findMyTransportScenario, index int) string {
	t.Helper()

	var value string

	decodeFindMyInput(t, scenario, index, &value)

	return value
}

func findMyBoolInput(t *testing.T, scenario findMyTransportScenario, index int) bool {
	t.Helper()

	var value bool

	decodeFindMyInput(t, scenario, index, &value)

	return value
}

func decodeFindMyInput(t *testing.T, scenario findMyTransportScenario, index int, value any) {
	t.Helper()

	if index >= len(scenario.Inputs) {
		t.Fatal("missing Find My scenario input")
	}

	err := json.Unmarshal(scenario.Inputs[index], value)
	if err != nil {
		t.Fatal(err)
	}
}

func assertFindMyDecoded(t *testing.T, value any, body []byte) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(accountJSON(t, encoded), accountJSON(t, body)) {
		t.Fatal("Find My typed reply lost fields, precision or null presence")
	}
}

func assertFindMyTransportResponse(t *testing.T, exchange replay.Exchange,
	response *webtransport.BytesResponse, err error, providerFailure bool,
) {
	t.Helper()

	expected := exchange.Response
	if expected == nil {
		t.Fatal("Find My transport outcome is missing")
	}

	if expected.Status >= http.StatusBadRequest || providerFailure {
		response = findMyFailureResponse(t, err)
	} else if err != nil {
		t.Fatal(err)
	}

	if response.Status != expected.Status || !reflect.DeepEqual(response.Body, findMyResponseBytes(t, expected.Body)) {
		t.Fatal("Find My response status or exact bytes changed")
	}

	actual := make(http.Header)
	for _, header := range expected.Headers {
		actual.Add(header[0], header[1])
	}

	if !reflect.DeepEqual(response.Headers, actual) ||
		response.CookieScopeURL != exchange.Request.Origin+exchange.Request.Path {
		t.Fatal("Find My response headers or cookie scope changed")
	}
}

func findMyFailureResponse(t *testing.T, err error) *webtransport.BytesResponse {
	t.Helper()

	var failure *webtransport.ResponseError
	if !errors.As(err, &failure) || failure.Stage != webtransport.Provider {
		t.Fatalf("Find My provider failure changed: %v", err)
	}

	return &webtransport.BytesResponse{CookieScopeURL: failure.CookieScopeURL,
		Body: failure.Body, Status: failure.Status, Headers: failure.Headers}
}

func findMyResponseBytes(t *testing.T, entity replay.Entity) []byte {
	t.Helper()

	var encoded string

	err := json.Unmarshal(entity.Value, &encoded)
	if err != nil {
		t.Fatal(err)
	}

	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	return body
}
