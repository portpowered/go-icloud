package replay_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	accountFamilyOperationName = "family"
	accountPlanOperationName   = "summary_plan"
)

type accountScenario struct {
	Operation string `json:"operation"`
	//nolint:tagliatelle // LIB-05: the portable fixture fixes this field spelling.
	Initial   accountInitial    `json:"initial_state"`
	Exchanges []replay.Exchange `json:"exchanges"`
	Result    json.RawMessage   `json:"result"`
	Error     json.RawMessage   `json:"error"`
}

type referenceCookie struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Domain  string `json:"domain"`
	Path    string `json:"path"`
	Secure  bool   `json:"secure"`
	Expires *int64 `json:"expires"`
}

type accountInitial struct {
	Cookies []referenceCookie `json:"cookies"`
	//nolint:tagliatelle // LIB-05: the portable fixture fixes this field spelling.
	DocumentOrigin string            `json:"document_origin"`
	Origin         string            `json:"origin"`
	Params         map[string]string `json:"params"`
	Headers        map[string]string `json:"headers"`
	//nolint:tagliatelle // LIB-05: the portable fixture fixes this field spelling.
	China bool `json:"china_mainland"`
}

func TestAccountSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(replayLiteralFixturesSyntheticHTTPAccountJSON)
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 34 {
		t.Fatal("account scenario inventory changed")
	}

	operations := make(map[string]int)

	for _, path := range paths {
		scenario := readAccountScenario(t, path)
		operations[scenario.Operation]++

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runAccountSDKScenario(t, scenario)
		})
	}

	if !maps.Equal(operations, map[string]int{
		replayDevicesOperation: 11, accountFamilyOperationName: 6,
		replayLiteralFamilyPhotos: 5, "storage": 4, accountPlanOperationName: 8,
	}) {
		t.Fatal("account operation scenario inventory changed", operations)
	}
}

func runAccountSDKScenario(t *testing.T, scenario accountScenario) {
	t.Helper()

	if scenario.Operation == replayDevicesOperation {
		accountDevicesSDK(t, scenario)

		return
	}

	runAccountSDKService(t, scenario)
}

func readAccountScenario(t *testing.T, path string) accountScenario {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var scenario accountScenario

	err = json.Unmarshal(data, &scenario)
	if err != nil {
		t.Fatal(err)
	}

	return scenario
}

func accountParameters[T any](t *testing.T, initial accountInitial) *T {
	t.Helper()

	values := make(map[string]string)
	maps.Copy(values, initial.Params)
	maps.Copy(values, initial.Headers)

	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}

	parameters := new(T)

	err = json.Unmarshal(data, parameters)
	if err != nil {
		t.Fatal(err)
	}

	return parameters
}

func accountJSON(t *testing.T, data []byte) any {
	t.Helper()

	var value any

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	err := decoder.Decode(&value)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

type accountPhotoResult struct {
	//nolint:tagliatelle // LIB-05: the portable result fixes this field spelling.
	MemberID string        `json:"member_id"`
	Status   int           `json:"status"`
	Headers  []replay.Pair `json:"headers"`
	Body     string        `json:"body"`
}

type accountFailureExpectation struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func accountProviderFailure(t *testing.T, encoded json.RawMessage, status int, body []byte) {
	t.Helper()

	var expected accountFailureExpectation

	err := json.Unmarshal(encoded, &expected)
	if err != nil {
		t.Fatal(err)
	}

	if expected.Type != replayExpectedPyiCloudAPIResponseException ||
		expected.Message != accountProviderMessage(t, status, body) {
		t.Fatal("account provider failure lost its recorded class/status/body")
	}
}

func accountProviderMessage(t *testing.T, status int, body []byte) string {
	t.Helper()

	if status != http.StatusOK {
		return fmt.Sprintf(" (%d): %s", status, body)
	}

	fields, ok := accountJSON(t, body).(map[string]any)
	if !ok {
		t.Fatal("provider envelope is not an object")
	}

	reason := "Unknown reason"

	for _, key := range []string{replayLiteralErrorMessage, "reason", "errorReason", "error"} {
		if text, matches := fields[key].(string); matches && text != "" {
			reason = text

			break
		}
	}

	return reason + ": " + string(body)
}
