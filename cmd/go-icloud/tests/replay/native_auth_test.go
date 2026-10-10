package replay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const expectedAccountDataKey = "accountData"

func TestNativeAuthenticationCommands(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ fixture, operation string }{
		{fixture: "auth-sms-request", operation: expectedMFARequestCommand},
		{fixture: "auth-sms-request-refused", operation: expectedMFARequestCommand},
		{fixture: "auth-sms-validate-accepted", operation: expectedMFAVerifyCommand},
		{fixture: "auth-sms-validate-refused", operation: expectedMFAVerifyCommand},
		{fixture: "auth-trust-success", operation: "trust"},
		{fixture: "auth-trust-refused", operation: "trust"},
		{fixture: "auth-trust-untrusted-account", operation: "trust"},
		{fixture: expectedCachedAuthFixture, operation: "login"},
		{fixture: "auth-status-trusted", operation: expectedAuthStatusCommand},
		{fixture: "auth-status-untrusted", operation: expectedAuthStatusCommand},
		{fixture: "auth-status-rejected", operation: expectedAuthStatusCommand},
		{fixture: "auth-status-no-token", operation: expectedAuthStatusCommand},
		{fixture: "auth-status-no-cookie", operation: expectedAuthStatusCommand},
		{fixture: "auth-trusted-devices-0", operation: expectedMFADevicesCommand},
		{fixture: "auth-trusted-devices-1", operation: expectedMFADevicesCommand},
		{fixture: "auth-trusted-devices-3", operation: expectedMFADevicesCommand},
		{fixture: "auth-logout-default", operation: "logout"},
		{fixture: "auth-logout-remote-error", operation: "logout"},
		{fixture: "auth-logout-remote-refused", operation: "logout"},
		{fixture: "auth-logout-no-cookie", operation: "logout"},
		{fixture: "auth-pcs-enabled", operation: expectedPCSAccessCommand},
		{fixture: "auth-pcs-consented", operation: expectedPCSAccessCommand},
		{fixture: "auth-pcs-consent-later", operation: expectedPCSAccessCommand},
		{fixture: "auth-pcs-consent-refused", operation: expectedPCSAccessCommand},
		{fixture: "auth-pcs-cookies-later", operation: expectedPCSAccessCommand},
		{fixture: "auth-pcs-retries-exhausted", operation: expectedPCSAccessCommand},
		{fixture: "auth-pcs-unknown-state", operation: expectedPCSAccessCommand},
		{fixture: "auth-pcs-consent-omitted", operation: expectedPCSAccessCommand},
		{fixture: "auth-pcs-consent-false", operation: expectedPCSAccessCommand},
		{fixture: "auth-pcs-consent-null", operation: expectedPCSAccessCommand},
	} {
		t.Run(test.fixture, func(t *testing.T) { t.Parallel(); runNativeAuthentication(t, test.fixture, test.operation) })
	}
}

func runNativeAuthentication(t *testing.T, name, operation string) {
	t.Helper()
	raw := readObject(t, filepath.Join("../../../../tests/replay/fixtures/synthetic/http", name+".json"))

	var exchanges []replay.Exchange

	decode(t, raw["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport),
		icloud.WithAuthenticationWait(func(ctx context.Context, _ time.Duration) error { return ctx.Err() }))
	if err != nil {
		t.Fatal(err)
	}

	state := nativeCommandState(t, raw["initial_state"])
	nativeSourceParameters(t, raw, &state)

	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "private.json")

	err = os.WriteFile(path, encoded, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	var inputs []string

	decode(t, raw["inputs"], &inputs)

	environment := func(string) string {
		if len(inputs) > 0 {
			return inputs[0]
		}

		return ""
	}

	var output, diagnostic bytes.Buffer

	arguments := []string{sessionFlag, path}
	if operation == expectedPCSAccessCommand {
		arguments = append(arguments, "--service", inputs[0])
	}

	arguments = append(arguments, operation)
	err = command.RunWithInput(t.Context(), client, arguments,
		io.NopCloser(strings.NewReader("")), environment, &output, &diagnostic)
	_, providerFailed := raw["error"]

	accepted := nativeCommandAccepted(t, raw, operation)

	if (providerFailed || !accepted) != (err != nil) {
		t.Fatalf("authentication acceptance differs: %v", err)
	}

	if providerFailed {
		var failure *icloud.ClientError
		if !errors.As(err, &failure) {
			t.Fatal("CLI lost the typed provider failure")
		}
	}

	assertNativeCommandPrivacy(t, output.String()+diagnostic.String(), []string{
		expectedSyntheticSessionValue, "synthetic-trust", expectedSyntheticAuthCookie,
		expectedSyntheticAccountName, expectedReplayResponses, expectedAccountDataKey})

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	assertNativeCommandPersistence(t, path, operation, providerFailed, state, encoded)
}

func assertNativeCommandPrivacy(t *testing.T, console string, secrets []string) {
	t.Helper()

	for _, secret := range secrets {
		if strings.Contains(console, secret) {
			t.Fatal("console disclosed private authentication data")
		}
	}
}

func assertNativeCommandPersistence(t *testing.T, path, operation string, providerFailed bool,
	state icloud.NativeAuthState, encoded []byte,
) {
	t.Helper()

	if operation == "logout" {
		_, readErr := os.Stat(path)
		if !os.IsNotExist(readErr) {
			t.Fatal("explicit logout retained local credentials")
		}

		return
	}

	if providerFailed {
		return
	}

	data, readErr := os.ReadFile(filepath.Clean(path))
	if readErr != nil {
		t.Fatal(readErr)
	}

	var saved icloud.NativeAuthState

	decode(t, data, &saved)

	if saved.Auth.ClientID != state.Auth.ClientID || saved.AccountName != state.AccountName {
		t.Fatal("private native state lost identity")
	}

	if operation == expectedPCSAccessCommand {
		savedState, marshalErr := json.Marshal(saved)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}

		if !bytes.Equal(savedState, encoded) {
			t.Fatal("PCS command changed the complete persisted authentication state")
		}
	}
}

func nativeCommandAccepted(t *testing.T, raw map[string]json.RawMessage, operation string) bool {
	t.Helper()

	result, exists := raw["result"]
	if !exists {
		return true
	}

	value := readRawObject(t, result)["value"]

	switch operation {
	case "login", expectedAuthStatusCommand, expectedMFADevicesCommand:
		return true
	case expectedPCSAccessCommand:
		if string(value) != "null" {
			t.Fatal("Source PCS completion must return None")
		}

		return true
	case "logout":
		var accepted bool

		decode(t, readRawObject(t, value)["remote_logout_confirmed"], &accepted)

		return accepted
	default:
		var accepted bool

		decode(t, value, &accepted)

		return accepted
	}
}

func readRawObject(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()

	var object map[string]json.RawMessage

	decode(t, raw, &object)

	return object
}

func nativeCommandState(t *testing.T, raw json.RawMessage) icloud.NativeAuthState {
	t.Helper()
	initial := readRawObject(t, raw)
	params := readRawObject(t, initial["params"])
	session := readRawObject(t, initial["session_data"])

	var state icloud.NativeAuthState

	state.AccountData = initial["account_data"]
	state.DeliveryMethod = icloud.TwoFactorDeliveryUnknown

	account := readRawObject(t, initial["account_data"])
	if information, exists := account["dsInfo"]; exists {
		if identifier, found := readRawObject(t, information)["dsid"]; found {
			decode(t, identifier, &state.Auth.AccountID)
		}
	}

	if delivery, exists := initial["delivery_method"]; exists {
		decode(t, delivery, &state.DeliveryMethod)
	}

	decode(t, initial["account_name"], &state.AccountName)
	decode(t, params["clientId"], &state.Auth.ClientID)
	state.Auth.SetupServiceURL = expectedAuthSetupURL

	if token, exists := session["session_token"]; exists {
		var value string

		decode(t, token, &value)
		state.Auth.SessionToken = &value
	}

	if trust, exists := session["trust_token"]; exists {
		decode(t, trust, &state.TrustToken)
	}

	if country, exists := session["account_country"]; exists {
		var value string

		decode(t, country, &value)
		state.AccountCountryCode.Set(value)
	}

	var headers map[string]string

	decode(t, initial["headers"], &headers)

	for name, value := range headers {
		state.Auth.Headers = append(state.Auth.Headers, icloud.Header{Name: name, Value: value})
	}

	var cookies []map[string]json.RawMessage

	decode(t, initial["cookies"], &cookies)

	for _, cookie := range cookies {
		var value icloud.AuthCookie

		decode(t, cookie["name"], &value.Name)
		decode(t, cookie["value"], &value.Value)
		decode(t, cookie["domain"], &value.Domain)
		decode(t, cookie["path"], &value.Path)
		value.HostOnly = !strings.HasPrefix(value.Domain, ".")
		state.Auth.Cookies = append(state.Auth.Cookies, value)
	}

	if challenge, exists := initial["auth_data"]; exists {
		object := readRawObject(t, challenge)
		if mode, ok := object["mode"]; ok {
			decode(t, mode, &state.Challenge.Mode)
		}

		if phone, ok := object["trustedPhoneNumber"]; ok {
			object = readRawObject(t, phone)

			var value icloud.TrustedPhoneNumber

			decode(t, object["id"], &value.ID)
			decode(t, object["numberWithDialCode"], &value.Number)
			decode(t, object["pushMode"], &value.PushMode)
			state.Challenge.PhoneNumbers = append(state.Challenge.PhoneNumbers, value)
		}
	}

	return state
}

func nativeSourceParameters(t *testing.T, raw map[string]json.RawMessage, state *icloud.NativeAuthState) {
	t.Helper()

	build, mastering := protocol.AuthClientBuildNumberValue, protocol.AuthClientMasteringNumberValue
	state.Auth.ClientBuildNumber = &build
	state.Auth.ClientMasteringNumber = &mastering

	parameters := readRawObject(t, readRawObject(t, raw["initial_state"])["params"])
	if build, found := parameters["clientBuildNumber"]; found {
		var value string

		decode(t, build, &value)
		state.Auth.ClientBuildNumber = &value
	}

	if mastering, found := parameters["clientMasteringNumber"]; found {
		var value string

		decode(t, mastering, &value)
		state.Auth.ClientMasteringNumber = &value
	}
}
