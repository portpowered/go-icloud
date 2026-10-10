package replay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const replayDevicesOperation = "devices"

type findMySDKInitial struct {
	findMyTransportInitial

	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	FamilyDelay *float64 `json:"family_poll_delay"`
	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	FamilyRetries *int `json:"family_poll_max_retries"`
}

type findMySDKEntropy struct {
	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	UnixSeconds float64 `json:"unix_seconds"`
	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	Waits []findMySDKWait `json:"findmy_wait_trace"`
	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	Monitor []findMyMonitorWait `json:"findmy_monitor_trace"`
}

type findMySDKWait struct {
	Kind  string  `json:"kind"`
	Value float64 `json:"value"`
}

type findMySDKScenario struct {
	Operation string `json:"operation"`
	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	Initial findMySDKInitial `json:"initial_state"`
	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	DeviceID  string            `json:"device_id"`
	Inputs    []json.RawMessage `json:"inputs"`
	Exchanges []replay.Exchange `json:"exchanges"`
	Result    json.RawMessage   `json:"result"`
	Error     json.RawMessage   `json:"error"`
	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	ErrorState json.RawMessage  `json:"error_findmy_state"`
	Entropy    findMySDKEntropy `json:"entropy"`
	//nolint:tagliatelle // LIB-05: portable provider-error response evidence.
	ErrorContext *findMyTransportErrorContext `json:"error_context"`
}

type findMySDKState struct {
	Devices []icloud.FindMyDevice `json:"devices"`
	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	UserInfo nullable.Nullable[icloud.FindMyUserInfo] `json:"user_info"`
	//nolint:tagliatelle // LIB-05: portable scenario spelling.
	ServerContext nullable.Nullable[json.RawMessage] `json:"server_context"`
}

type findMyReplayScheduler struct {
	t     *testing.T
	waits []findMySDKWait
	index int
}

func (scheduler *findMyReplayScheduler) Wait(ctx context.Context, request icloud.FindMyWaitRequest) error {
	scheduler.t.Helper()

	if ctx.Err() != nil {
		return fmt.Errorf("wait for Find My replay: %w", ctx.Err())
	}

	if request.Kind != icloud.FindMyFamilyPollWait || scheduler.index >= len(scheduler.waits) {
		scheduler.t.Fatal("undeclared Find My lifecycle wait")
	}

	expected := scheduler.waits[scheduler.index]
	if expected.Kind != "sleep" || request.Delay.Seconds() != expected.Value {
		scheduler.t.Fatal("Find My wait duration or event kind changed")
	}

	scheduler.index++

	return nil
}

func TestFindMySDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(replayLiteralFixturesSyntheticHTTPFindmyJSON)
	if err != nil {
		t.Fatal(err)
	}

	pairs := 0
	selected := 0

	for _, path := range paths {
		scenario := readFindMySDKScenario(t, path)
		if scenario.Operation == "devices_with_auth_state" {
			continue // The cross-service token lifecycle is bound by TestFindMySavedTokenRecoverySDK.
		}

		selected++
		pairs += len(scenario.Exchanges)

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runFindMySDK(t, scenario)
		})
	}

	if selected != 71 || pairs != 140 {
		t.Fatal("Find My SDK scenario inventory changed")
	}
}

func readFindMySDKScenario(t *testing.T, path string) findMySDKScenario {
	t.Helper()

	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var scenario findMySDKScenario

	err = json.Unmarshal(body, &scenario)
	if err != nil {
		t.Fatal(err)
	}

	return scenario
}

func runFindMySDK(t *testing.T, scenario findMySDKScenario) {
	t.Helper()

	if scenario.Operation == "monitor_flow" {
		runFindMyMonitorSDK(t, scenario)

		return
	}

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	scheduler := &findMyReplayScheduler{t: t, waits: scenario.Entropy.Waits, index: 0}
	auth := findMySDKAuth(t, scenario.Initial)

	delay, retries := findMySDKPolling(scenario.Initial)

	session, callErr := client.OpenFindMySession(t.Context(),
		icloud.OpenFindMySessionRequest{Auth: auth, IncludeFamily: scenario.Initial.Family},
		icloud.WithFindMyMonitorInterval(0), icloud.WithFindMyFamilyPolling(delay, retries),
		icloud.WithFindMyScheduler(scheduler))
	if session != nil {
		t.Cleanup(func() {
			closeErr := session.Close()
			if closeErr != nil {
				t.Error(closeErr)
			}
		})
	}

	cursor := 0

	var result any

	if callErr == nil {
		cursor = checkFindMySDKResponses(t, scenario, cursor, session.LastResponses())
		result, callErr = callFindMySDK(t, scenario, session, &cursor)
	}

	cursor = checkFindMySDKOutcome(t, scenario, session, cursor, result, callErr)

	if scheduler.index != len(scheduler.waits) || cursor != len(scenario.Exchanges) {
		t.Fatal("Find My SDK left unconsumed waits or response evidence")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func findMySDKAuth(t *testing.T, initial findMySDKInitial) icloud.AuthContext {
	t.Helper()

	auth := sdkAccountAuth(initial.accountInitial)
	auth.AccountServiceURL = ""
	auth.FindMyServiceURL = initial.Origin

	target, err := url.Parse(initial.TokenOrigin)
	if err != nil {
		t.Fatal(err)
	}

	auth.SetupServiceURL = target.Scheme + "://" + target.Host
	if token, exists := initial.SessionData[replayExpectedSessionStateField]; exists {
		auth.SessionToken = &token
	}

	return auth
}

func callFindMySDK(t *testing.T, scenario findMySDKScenario, session *icloud.FindMySession,
	cursor *int,
) (any, error) {
	t.Helper()

	if scenario.Operation == replayDevicesOperation {
		return findMySDKSnapshot(t, session).Devices, nil
	}

	if scenario.Operation == "device_description" {
		return callFindMyDescription(t, scenario, session)
	}

	if scenario.Operation == "refresh_flow" {
		states := []findMySDKState{findMySDKSnapshot(t, session)}

		for _, input := range scenario.Inputs {
			var request icloud.RefreshFindMyRequest

			decodeErr := json.Unmarshal(input, &request)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}

			_, err := session.Refresh(t.Context(), request)
			if err != nil {
				return nil, fmt.Errorf("refresh Find My scenario: %w", err)
			}

			*cursor = checkFindMySDKResponses(t, scenario, *cursor, session.LastResponses())
			states = append(states, findMySDKSnapshot(t, session))
		}

		return states, nil
	}

	result, err := callFindMySDKCommand(t, scenario, session)
	if err != nil {
		return nil, err
	}

	*cursor = checkFindMySDKResponses(t, scenario, *cursor, result.Responses)
	if !bytes.Equal(result.Acknowledgement,
		findMyResponseBytes(t, scenario.Exchanges[*cursor-1].Response.Body)) {
		t.Fatal("Find My command discarded acknowledgement bytes")
	}

	return json.RawMessage(reminderChangeNullValue), nil
}

func callFindMySDKCommand(t *testing.T, scenario findMySDKScenario,
	session *icloud.FindMySession,
) (*icloud.FindMyCommandResult, error) {
	t.Helper()

	inputs := findMyTransportScenario{Initial: scenario.Initial.findMyTransportInitial,
		Exchanges: nil, Inputs: scenario.Inputs, DeviceID: scenario.DeviceID, ErrorContext: scenario.ErrorContext}

	var (
		result *icloud.FindMyCommandResult
		err    error
	)

	switch scenario.Operation {
	case "play_sound":
		subject := findMyStringInput(t, inputs, 0)
		result, err = session.PlaySound(t.Context(), icloud.FindMySoundRequest{
			DeviceID: scenario.DeviceID, Subject: &subject})
	case "display_message":
		subject, text := findMyStringInput(t, inputs, 0), findMyStringInput(t, inputs, 1)
		result, err = session.SendMessage(t.Context(), icloud.FindMyMessageRequest{DeviceID: scenario.DeviceID,
			Subject: &subject, Text: &text, Sound: findMyBoolInput(t, inputs, 2),
			Vibrate: findMyBoolInput(t, inputs, 3), Strobe: findMyBoolInput(t, inputs, 4)})
	case "lost_device":
		text := findMyStringInput(t, inputs, 1)
		result, err = session.MarkLost(t.Context(), icloud.FindMyLostRequest{DeviceID: scenario.DeviceID,
			PhoneNumber: findMyStringInput(t, inputs, 0), Text: &text, Passcode: findMyStringInput(t, inputs, 2)})
	case "erase_device":
		request := icloud.FindMyEraseRequest{DeviceID: scenario.DeviceID, Text: nil, Passcode: ""}

		if len(scenario.Inputs) != 0 {
			text := findMyStringInput(t, inputs, 0)
			request.Text, request.Passcode = &text, findMyStringInput(t, inputs, 1)
		}

		result, err = session.Erase(t.Context(), request)
	default:
		t.Fatal("unknown Find My SDK command")
	}

	if err != nil {
		return nil, fmt.Errorf("Find My SDK command: %w", err)
	}

	return result, nil
}

func findMySDKSnapshot(t *testing.T, session *icloud.FindMySession) findMySDKState {
	t.Helper()

	if session == nil {
		t.Fatal("Find My scenario expected an opened session")
	}

	snapshot, err := session.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	return findMySDKState{Devices: snapshot.Devices, UserInfo: snapshot.UserInfo, ServerContext: snapshot.ServerContext}
}

func assertFindMySDKJSON(t *testing.T, value any, expected json.RawMessage) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(accountJSON(t, encoded), accountJSON(t, expected)) {
		t.Fatalf("Find My public result changed: %s", encoded)
	}
}

func checkFindMySDKResponses(t *testing.T, scenario findMySDKScenario, cursor int,
	metadata []icloud.ResponseMetadata,
) int {
	t.Helper()

	for _, response := range metadata {
		if cursor >= len(scenario.Exchanges) {
			t.Fatal("Find My response evidence repeated or fabricated")
		}

		checkSDKMetadata(t, response, scenario.Exchanges[cursor].Response)

		if response.CookieScopeURL != scenario.Exchanges[cursor].Request.Origin+scenario.Exchanges[cursor].Request.Path {
			t.Fatal("Find My response cookie scope changed")
		}

		cursor++
	}

	return cursor
}

func checkFindMySDKError(t *testing.T, scenario findMySDKScenario, cursor int, err error) int {
	t.Helper()

	var failure *icloud.ClientError
	if !errors.As(err, &failure) {
		t.Fatalf("Find My lost typed failure: %v", err)
	}

	var expected accountFailureExpectation

	decodeErr := json.Unmarshal(scenario.Error, &expected)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	if failure.Kind() != findMySDKFailureKind(scenario, expected.Type) {
		t.Fatalf("Find My refusal classification changed: %v", err)
	}

	cursor = checkFindMySDKResponses(t, scenario, cursor, failure.PriorResponses())
	if failure.StatusCode() != 0 {
		cursor = checkFindMySDKResponses(t, scenario, cursor, []icloud.ResponseMetadata{{
			CookieScopeURL: failure.CookieScopeURL(), StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(),
		}})
		if !reflect.DeepEqual(failure.ResponseBody(), findMyResponseBytes(t, scenario.Exchanges[cursor-1].Response.Body)) {
			t.Fatal("Find My error response bytes changed")
		}
	} else if len(failure.ResponseBody()) != 0 || len(failure.ResponseHeaders()) != 0 {
		t.Fatal("local Find My refusal invented a provider response")
	}

	return cursor
}

func findMySDKPolling(initial findMySDKInitial) (time.Duration, int) {
	delay, retries := 500*time.Millisecond, 5
	if initial.FamilyDelay != nil {
		delay = time.Duration(*initial.FamilyDelay * float64(time.Second))
	}

	if initial.FamilyRetries != nil {
		retries = *initial.FamilyRetries
	}

	return delay, retries
}

func checkFindMySDKOutcome(t *testing.T, scenario findMySDKScenario, session *icloud.FindMySession,
	cursor int, result any, callErr error,
) int {
	t.Helper()

	if len(scenario.Error) != 0 {
		cursor = checkFindMySDKError(t, scenario, cursor, callErr)

		if len(scenario.ErrorState) != 0 {
			assertFindMySDKJSON(t, findMySDKSnapshot(t, session), scenario.ErrorState)
		}
	} else {
		if callErr != nil {
			t.Fatal(callErr)
		}

		assertFindMySDKJSON(t, result, scenario.Result)
	}

	return cursor
}

func callFindMyDescription(t *testing.T, scenario findMySDKScenario, session *icloud.FindMySession) (any, error) {
	t.Helper()

	var request icloud.FindMyDeviceDescriptionRequest

	err := json.Unmarshal(scenario.Inputs[0], &request)
	if err != nil {
		t.Fatal(err)
	}

	request.DeviceID = scenario.DeviceID

	value, err := session.DescribeDevice(request)
	if err != nil {
		return nil, fmt.Errorf("describe Find My scenario: %w", err)
	}

	return value, nil
}

func findMySDKFailureKind(scenario findMySDKScenario, sourceType string) icloud.ErrorKind {
	if sourceType == "PyiCloudNoDevicesException" {
		return icloud.NoDevices
	}

	if scenario.ErrorContext != nil && scenario.ErrorContext.Response != nil &&
		scenario.ErrorContext.Response.Status == http.StatusOK {
		return icloud.Provider
	}

	return icloud.Unavailable
}
