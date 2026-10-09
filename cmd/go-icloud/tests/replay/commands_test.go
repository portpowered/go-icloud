// Package replay_test verifies CLI requests against the canonical portable fixtures.
package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const driveNodeCommand = "drive-node"

const accountDevicesCommand = "account-devices"

const sessionFlag = "--session"

const referenceStateFlag = "--reference-state"

const saveSessionFlag = "--save-session"

const resumeCommand = "resume"

func TestReadCommands(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		accountDevicesCommand: {
			"account-devices-empty", "account-devices-many", "account-devices-one",
			"account-devices-error", "account-devices-envelope-reason",
		},
		"account-family":  {"account-family-empty", "account-family-many", "account-family-error"},
		"account-storage": {"account-storage-zero", "account-storage-media", "account-storage-error"},
		"account-plan": {
			"account-summary-empty", "account-summary-array", "account-summary-error", "account-summary-china-plan",
		},
		"drive-libraries": {"drive-apps-empty", "drive-apps-many", "drive-apps-one-refused-0"},
		driveNodeCommand:  {"drive-folder-empty", "drive-folder-many", "drive-folder-one-refused-0"},
		"findmy":          {"findmy-devices-empty", "findmy-devices-many", "findmy-devices-one-refused-0"},
	}
	for operation, fixtures := range cases {
		for _, name := range fixtures {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				runFixture(t, operation, name)
			})
		}
	}
}

// This is an explicit synthetic HTTP control, not another Source-derived scenario.
func TestAuthenticationRefusal(t *testing.T) {
	t.Parallel()
	raw := readObject(t, "../../../../tests/replay/fixtures/synthetic/http/account-devices-empty.json")

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)
	exchanges[0].Response.Status = 401

	encoded, err := json.Marshal(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	raw["exchanges"] = encoded
	raw["error"] = json.RawMessage(`{"type":"synthetic authentication refusal"}`)
	runScenario(t, accountDevicesCommand, raw)
}

func runFixture(t *testing.T, operation, name string) {
	t.Helper()
	raw := readObject(t, filepath.Join("../../../../tests/replay/fixtures/synthetic/http", name+".json"))

	runScenario(t, operation, raw)
}

func runScenario(t *testing.T, operation string, raw map[string]json.RawMessage) {
	t.Helper()

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	session := filepath.Join(t.TempDir(), "session.json")

	//nolint:gosec // Only synthetic fixture credentials are serialized into a temporary test directory.
	encoded, err := json.Marshal(fixtureAuth(t, raw["initial_state"]))
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(session, encoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	args := []string{sessionFlag, session}

	if operation == driveNodeCommand {
		var inputs []string

		decode(t, raw["inputs"], &inputs)
		args = append(args, "--node", inputs[0])
	}

	args = append(args, operation)

	var output, diagnostic bytes.Buffer

	err = command.Run(t.Context(), client, args, &output, &diagnostic)
	assertOutcome(t, operation, raw, output.Bytes(), err)

	if diagnostic.Len() != 0 {
		t.Fatal("read printed unexpected diagnostic output")
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func fixtureAuth(t *testing.T, initial json.RawMessage) icloud.AuthContext {
	t.Helper()

	var state map[string]json.RawMessage

	decode(t, initial, &state)

	var params, headers map[string]string

	decode(t, state["params"], &params)
	decode(t, state["headers"], &headers)

	var origin string

	decode(t, state["origin"], &origin)

	var auth icloud.AuthContext

	auth.AccountID = params["dsid"]
	auth.ClientID = params["clientId"]
	auth.AccountServiceURL = origin
	auth.DriveServiceURL = origin

	auth.FindMyServiceURL = origin
	auth.RemindersServiceURL = origin

	for name, value := range headers {
		auth.Headers = append(auth.Headers, icloud.Header{Name: name, Value: value})
	}

	if value, exists := params["clientBuildNumber"]; exists {
		auth.ClientBuildNumber = &value
	}

	if value, exists := params["clientMasteringNumber"]; exists {
		auth.ClientMasteringNumber = &value
	}

	if region, exists := state["china_mainland"]; exists {
		var china bool

		decode(t, region, &china)
		auth.ChinaMainland = &china
	}

	return auth
}

func assertOutcome(t *testing.T, operation string, raw map[string]json.RawMessage, output []byte, err error) {
	t.Helper()

	if _, failed := raw["error"]; failed {
		var failure *icloud.ClientError
		if !errors.As(err, &failure) || len(output) != 0 {
			t.Fatalf("CLI did not preserve typed failure: %v", err)
		}

		assertFailure(t, raw, failure)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	var actual any

	decode(t, output, &actual)

	if operation != "findmy" {
		var object map[string]json.RawMessage

		decode(t, output, &object)

		if _, exists := object["metadata"]; exists {
			t.Fatal("CLI exposed response headers")
		}

		actual = projectResult(t, operation, object)
	}

	var expected any

	decode(t, raw["result"], &expected)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("CLI changed reference result: got %v want %v", actual, expected)
	}
}

func projectResult(t *testing.T, operation string, object map[string]json.RawMessage) any {
	t.Helper()

	fields := map[string]string{
		accountDevicesCommand: "devices", "account-plan": "summary",
		"drive-libraries": "libraries", driveNodeCommand: "node",
	}
	if field, exists := fields[operation]; exists {
		var value any

		decode(t, object[field], &value)

		return value
	}

	if operation == "account-family" {
		var members []map[string]json.RawMessage

		decode(t, object["members"], &members)

		names := make([]any, 0, len(members))

		for _, member := range members {
			var name any

			decode(t, member["fullName"], &name)
			names = append(names, name)
		}

		return names
	}

	var usage map[string]any

	decode(t, object["usage"], &usage)

	return map[string]any{"used_bytes": usage["usedStorageInBytes"], "total_bytes": usage["totalStorageInBytes"]}
}

func readObject(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var object map[string]json.RawMessage

	decode(t, data, &object)

	return object
}

func decode(t *testing.T, data []byte, result any) {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	err := decoder.Decode(result)
	if err != nil {
		t.Fatal(err)
	}
}

func assertFailure(t *testing.T, raw map[string]json.RawMessage, failure *icloud.ClientError) {
	t.Helper()

	var sourceError map[string]string

	decode(t, raw["error"], &sourceError)

	if sourceError["type"] == "PyiCloudNoDevicesException" {
		if failure.Kind() != icloud.NoDevices {
			t.Fatal("CLI lost empty-discovery failure class")
		}

		if failure.StatusCode() != 0 || len(failure.ResponseBody()) != 0 {
			t.Fatal("empty discovery changed")
		}

		return
	}

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)
	response := exchanges[0].Response

	var encoded string

	decode(t, response.Body.Value, &encoded)

	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if failure.StatusCode() != response.Status || !bytes.Equal(failure.ResponseBody(), body) {
		t.Fatal("CLI lost provider status or body")
	}

	expectedKind := expectedFailureKind(response.Status)
	if failure.Kind() != expectedKind {
		t.Fatal("CLI lost provider failure class")
	}

	if strings.Contains(failure.Error(), string(body)) {
		t.Fatal("CLI display error exposed provider content")
	}
}

func expectedFailureKind(status int) icloud.ErrorKind {
	switch status {
	case 401:
		return icloud.Unauthorized
	case 503:
		return icloud.Unavailable
	default:
		return icloud.Provider
	}
}
