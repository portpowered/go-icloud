package replay_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type driveNodeScenario struct {
	driveUploadScenario

	//nolint:tagliatelle // LIB-05: portable selector spelling.
	Selector string `json:"node_selector"`
	//nolint:tagliatelle // LIB-05: portable failure-state spelling.
	NodeError json.RawMessage `json:"error_node_state"`
}

type driveNodeCall struct {
	Operation string                     `json:"operation"`
	Inputs    []json.RawMessage          `json:"inputs"`
	Keywords  map[string]json.RawMessage `json:"kwargs"`
}

type driveNodeRunner struct {
	scenario driveNodeScenario
	session  *icloud.DriveSession
	root     *icloud.DriveEntry
	node     *icloud.DriveEntry
	traffic  *nodeReplayTraffic
}

type nodeReplayTraffic struct {
	base  *replay.HTTPTransport
	count int
}

func (traffic *nodeReplayTraffic) RoundTrip(request *http.Request) (*http.Response, error) {
	traffic.count++

	response, err := traffic.base.RoundTrip(request)
	if err != nil {
		return response, fmt.Errorf("node traffic: %w", err)
	}

	return response, nil
}

func TestDriveSDKPortableNodes(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(replayLiteralFixturesSyntheticHTTPDriveJSON)
	if err != nil {
		t.Fatal(err)
	}

	selected := 0

	for _, path := range paths {
		data, readErr := os.ReadFile(filepath.Clean(path))
		if readErr != nil {
			t.Fatal(readErr)
		}

		var scenario driveNodeScenario

		decodeErr := json.Unmarshal(data, &scenario)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		if scenario.Operation != "node_flow" {
			continue
		}

		selected++

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runDriveNodes(t, scenario)
		})
	}

	if selected != 32 {
		t.Fatal("Drive node-flow inventory changed", selected)
	}
}

func runDriveNodes(t *testing.T, scenario driveNodeScenario) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	traffic := &nodeReplayTraffic{base: transport, count: 0}

	client, err := icloud.New(icloud.WithHTTPTransport(traffic), icloud.WithClock(func() time.Time {
		return time.Unix(scenario.Entropy.Seconds, 0)
	}))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.DriveServiceURL = scenario.Initial.Origin
	auth.DriveDocumentServiceURL = scenario.Initial.DocumentOrigin

	session, err := client.OpenDriveSession(t.Context(), icloud.OpenDriveSessionRequest{Auth: auth})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := session.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	runner := driveNodeRunner{scenario: scenario, session: session, root: nil, node: nil, traffic: traffic}

	result, err := runner.execute(t)
	if len(scenario.Error) != 0 {
		checkDriveNodeFailureState(t, scenario, &runner, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkSDKValue(t, result, scenario.Result)
	}

	consumeErr := transport.AssertConsumed()
	if consumeErr != nil {
		t.Fatal(consumeErr)
	}
}

func (runner *driveNodeRunner) execute(t *testing.T) (any, error) {
	t.Helper()

	var err error
	if runner.scenario.Selector == nodeTrashLabel {
		runner.root, err = runner.session.Trash(t.Context(), icloud.DriveLocationRequest{Refresh: false})
	} else {
		runner.root, err = runner.session.Root(t.Context(), icloud.DriveLocationRequest{Refresh: false})
	}

	if err != nil {
		return nil, fmt.Errorf(replayNodeErrorFormat, err)
	}

	runner.node = runner.root
	checkNodeResponseMetadata(t, runner, 0)

	values := make([]any, 0, len(runner.scenario.Inputs))

	for _, raw := range runner.scenario.Inputs {
		var call driveNodeCall

		decodeErr := json.Unmarshal(raw, &call)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		start := runner.traffic.count

		value, callErr := runner.call(t, call)
		checkNodeResponseMetadata(t, runner, start)

		if callErr != nil {
			return nil, callErr
		}

		values = append(values, value)
	}

	return map[string]any{"values": values, "node": portableDriveSnapshot(t, runner.node),
		nodeRootLabel: portableDriveSnapshot(t, runner.root)}, nil
}

func checkNodeResponseMetadata(t *testing.T, runner *driveNodeRunner, start int) {
	t.Helper()

	if runner.traffic.count == start {
		return
	}

	metadata := runner.session.LastResponses()
	if len(metadata) != runner.traffic.count-start {
		t.Fatalf("node operation lost response-stage evidence: start=%d calls=%d metadata=%d",
			start, runner.traffic.count, len(metadata))
	}

	for index, value := range metadata {
		checkSDKMetadata(t, value, runner.scenario.Exchanges[start+index].Response)
	}
}

func checkDriveNodeError(t *testing.T, scenario driveNodeScenario, err error) {
	t.Helper()

	var source struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}

	decodeErr := json.Unmarshal(scenario.Error, &source)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	var failure *icloud.ClientError
	if !errors.As(err, &failure) {
		t.Fatalf("node failure is not typed: %v", err)
	}

	if source.Type == replayExpectedPyiCloudAPIResponseException {
		if failure.Kind() != icloud.Unavailable {
			t.Fatal("node upload refusal classification changed")
		}

		accountProviderFailure(t, scenario.Error, failure.StatusCode(), failure.ResponseBody())

		return
	}

	want := driveNodeLocalKind(source.Type, source.Message)
	if failure.Kind() != want {
		t.Fatal("node local refusal classification changed", source.Type, failure.Kind())
	}

	if failure.StatusCode() != 0 || len(failure.ResponseBody()) != 0 || len(failure.ResponseHeaders()) != 0 {
		t.Fatal("local node refusal invented a provider response")
	}
}

func hasDriveNodeUpload(scenario driveNodeScenario) bool {
	for _, raw := range scenario.Inputs {
		var call driveNodeCall

		decodeErr := json.Unmarshal(raw, &call)
		if decodeErr == nil && call.Operation == "upload" {
			return true
		}
	}

	return false
}

func driveNodeLocalKind(kind, message string) icloud.ErrorKind {
	want := icloud.NotFound

	switch kind {
	case "NotADirectoryError":
		want = icloud.NotDirectory
	case replayExpectedSourceValueError:
		want = icloud.NotInTrash
	}

	if message == "'No items in folder, status: NOT_FOUND'" {
		want = icloud.InvalidResponse
	}

	return want
}

func checkDriveNodeFailureState(t *testing.T, scenario driveNodeScenario, runner *driveNodeRunner, err error) {
	t.Helper()
	checkDriveNodeError(t, scenario, err)
	checkSDKValue(t, map[string]any{nodeRootLabel: portableDriveSnapshot(t, runner.root),
		"node": portableDriveSnapshot(t, runner.node)}, scenario.NodeError)

	if !hasDriveNodeUpload(scenario) {
		params := maps.Clone(scenario.Initial.Params)
		if token := runner.session.Authentication().DriveToken; token != "" {
			params["token"] = token
		}

		checkSDKValue(t, map[string]any{"params": params, replayLiteralFilePosition: nil}, scenario.ErrorState)
	}
}
