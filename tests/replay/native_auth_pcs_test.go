package replay_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestNativePCSReplay(t *testing.T) {
	t.Parallel()

	for _, scenario := range []string{"auth-pcs-enabled", "auth-pcs-consented", "auth-pcs-consent-later",
		"auth-pcs-consent-refused", "auth-pcs-cookies-later", "auth-pcs-retries-exhausted", "auth-pcs-unknown-state",
		"auth-pcs-consent-omitted", "auth-pcs-consent-false", "auth-pcs-consent-null"} {
		t.Run(scenario, func(t *testing.T) { t.Parallel(); nativePCSReplay(t, scenario) })
	}
}

func nativePCSReplay(t *testing.T, name string) {
	t.Helper()

	raw, transport, state := nativeFlowFixture(t, name)
	waits := 0
	wait := func(ctx context.Context, duration time.Duration) error {
		if duration != 5*time.Second {
			t.Fatalf("unexpected PCS interval %s", duration)
		}
		waits++
		return ctx.Err()
	}
	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithAuthenticationWait(wait))
	if err != nil {
		t.Fatal(err)
	}

	var inputs []string
	authReplayDecode(t, raw["inputs"], &inputs)
	request := icloud.RequestPCSAccessRequest{Auth: state.Auth, State: state, Service: inputs[0]}
	result, err := client.RequestPCSAccess(t.Context(), request)
	nativeFlowExpectedError(t, raw, err)
	if err == nil {
		if !result.Success {
			t.Fatal("completed Source PCS operation was not successful")
		}
		if !reflect.DeepEqual(accountJSON(t, result.State.AccountData), accountJSON(t, state.AccountData)) {
			t.Fatal("PCS changed account discovery")
		}

		nativeFlowResponses(t, raw, result.Responses)
	}
	expected := map[string]int{"auth-pcs-consent-later": 1, "auth-pcs-cookies-later": 1, "auth-pcs-retries-exhausted": 10,
		"auth-pcs-consent-false": 1, "auth-pcs-consent-null": 1}
	if waits != expected[name] {
		t.Fatalf("wait count %d, expected %d", waits, expected[name])
	}
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
}

func TestNativePCSAccountProjectionFailure(t *testing.T) {
	t.Parallel()

	raw, transport, state := nativeFlowFixture(t, "auth-pcs-enabled")
	state.AccountData = json.RawMessage(`{`)
	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.RequestPCSAccess(t.Context(), icloud.RequestPCSAccessRequest{
		Auth: state.Auth, State: state, Service: protocol.AuthWebServicesPhotos})
	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse {
		t.Fatalf("expected malformed account projection failure, got %v", err)
	}
	var exchanges []replay.Exchange
	authReplayDecode(t, raw["exchanges"], &exchanges)
	final := exchanges[len(exchanges)-1]
	if failure.StatusCode() != final.Response.Status ||
		!reflect.DeepEqual(failure.ResponseBody(), nativeFlowBody(t, final.Response.Body)) ||
		failure.CookieScopeURL() != final.Request.Origin+final.Request.Path {
		t.Fatal("account projection failure lost completed PCS response evidence")
	}
	metadata := append(failure.PriorResponses(), icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()})
	nativeFlowResponses(t, raw, metadata)
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
}

func nativeFlowFixture(t *testing.T, name string) (map[string]json.RawMessage, *replay.HTTPTransport, icloud.NativeAuthState) {
	t.Helper()

	raw := authReplayObject(t, filepath.Join("fixtures/synthetic/http", name+".json"))

	var exchanges []replay.Exchange

	authReplayDecode(t, raw["exchanges"], &exchanges)
	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}
	initial := authReplayObjectBytes(t, raw["initial_state"])
	adapted := maps.Clone(raw)
	adapted["keyword_inputs"] = json.RawMessage(`{}`)
	resume := authReplayRequest(t, adapted)
	country := resume.AccountCountryCode
	if !country.IsSpecified() {
		country.SetNull()
	}

	var boundary icloud.AuthContext
	boundary.ClientID = resume.Auth.ClientID
	boundary.ChinaMainland = resume.Auth.ChinaMainland
	boundary.Cookies = resume.Auth.Cookies
	boundary.Headers = resume.Auth.Headers
	boundary.SetupServiceURL = resume.Auth.SetupServiceURL
	state := icloud.NativeAuthState{Auth: boundary, AccountName: "", AcceptTerms: false, DeliveryNotice: nil, AccountCountryCode: country,
		AccountData: initial["account_data"], TrustToken: resume.TrustToken, Challenge: icloud.NativeAuthChallenge{
			Mode: "", AuthInitialRoute: "", HasTrustedDevices: false, PhoneNumbers: []icloud.TrustedPhoneNumber{},
			AuthFactors: []string{}, SecurityKeyNames: []string{}, BridgeBootstrap: nil, SecurityKeyChallenge: nil, ProviderData: nil},
		CodeRequested: false, RequiresMFA: false, DeliveryMethod: icloud.TwoFactorDeliveryUnknown}
	authReplayDecode(t, initial["account_name"], &state.AccountName)

	var account auth.AuthAccountResponse
	authReplayDecode(t, initial["account_data"], &account)
	if account.DsInfo != nil && account.DsInfo.Dsid != nil {
		state.Auth.AccountID = *account.DsInfo.Dsid
	}
	if account.Webservices != nil {
		services := account.Webservices
		state.Auth.AccountServiceURL = nativeFixtureService(services.Account)
		state.Auth.DriveServiceURL = nativeFixtureService(services.Drivews)
		state.Auth.DriveDocumentServiceURL = nativeFixtureService(services.Docws)
		state.Auth.FindMyServiceURL = nativeFixtureService(services.Findme)
		state.Auth.PhotosServiceURL = nativeFixtureService(services.Ckdatabasews)
		state.Auth.RemindersServiceURL = nativeFixtureService(services.Ckdatabasews)
		state.Auth.LegacyRemindersServiceURL = nativeFixtureService(services.Reminders)
		state.Auth.PhotosUploadServiceURL = nativeFixtureService(services.Photosupload)
		state.Auth.SharedPhotosServiceURL = nativeFixtureService(services.Sharedstreams)
	}
	build, mastering := protocol.AuthClientBuildNumberValue, protocol.AuthClientMasteringNumberValue
	state.Auth.ClientBuildNumber = &build
	state.Auth.ClientMasteringNumber = &mastering
	if resume.Auth.SessionToken != "" {
		token := resume.Auth.SessionToken
		state.Auth.SessionToken = &token
	}
	return raw, transport, state
}

func authReplayObjectBytes(t *testing.T, value json.RawMessage) map[string]json.RawMessage {
	t.Helper()

	var object map[string]json.RawMessage
	authReplayDecode(t, value, &object)
	return object
}

func nativeFlowExpectedError(t *testing.T, raw map[string]json.RawMessage, err error) {
	t.Helper()

	_, expected := raw["error"]
	if expected != (err != nil) {
		t.Fatalf("expected error %v, got %v", expected, err)
	}
	if err == nil {
		return
	}

	var failure *icloud.ClientError
	if !errors.As(err, &failure) {
		t.Fatalf("untyped authentication error %T", err)
	}

	var exchanges []replay.Exchange

	authReplayDecode(t, raw["exchanges"], &exchanges)
	last := exchanges[len(exchanges)-1].Response
	if failure.StatusCode() != last.Status || !reflect.DeepEqual(failure.ResponseBody(), nativeFlowBody(t, last.Body)) {
		t.Fatal("authentication failure lost final response evidence")
	}
	if len(failure.PriorResponses()) != len(exchanges)-1 {
		t.Fatal("authentication failure lost prior response metadata")
	}

	metadata := append(failure.PriorResponses(), icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()})
	nativeFlowResponses(t, raw, metadata)
	if failure.CookieScopeURL() != exchanges[len(exchanges)-1].Request.Origin+exchanges[len(exchanges)-1].Request.Path {
		t.Fatal("authentication failure lost cookie scope")
	}
}

func nativeFlowBody(t *testing.T, body replay.Entity) []byte {
	t.Helper()

	return contractAuthBody(t, body)
}

func nativeFlowResponses(t *testing.T, raw map[string]json.RawMessage, actual []icloud.ResponseMetadata) {
	t.Helper()

	var result icloud.ResumeSessionResult
	result.Responses = actual
	assertAuthResponses(t, raw, &result)
}

func TestNativePCSWaitCancellation(t *testing.T) {
	t.Parallel()

	raw, _, state := nativeFlowFixture(t, "auth-pcs-consent-later")

	var exchanges []replay.Exchange

	authReplayDecode(t, raw["exchanges"], &exchanges)
	transport, err := replay.NewHTTPTransport(exchanges[:2])
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	wait := func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }
	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithAuthenticationWait(wait))
	if err != nil {
		t.Fatal(err)
	}
	request := icloud.RequestPCSAccessRequest{Auth: state.Auth, State: state, Service: protocol.AuthWebServicesPhotos}
	_, err = client.RequestPCSAccess(ctx, request)

	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != icloud.Canceled || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellable PCS wait, got %v", err)
	}
	if len(failure.PriorResponses()) != 2 {
		t.Fatal("cancelled wait lost prior responses")
	}
	if err = transport.AssertConsumed(); err != nil {
		t.Fatal(err)
	}
}

func TestNativePCSInvalidResponses(t *testing.T) {
	t.Parallel()

	for _, body := range []string{"null", "[]", "{", `{"isICDRSDisabled":"invalid"}`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			raw, _, state := nativeFlowFixture(t, "auth-pcs-enabled")
			var exchanges []replay.Exchange

			authReplayDecode(t, raw["exchanges"], &exchanges)
			encoded, marshalErr := json.Marshal(base64.StdEncoding.EncodeToString([]byte(body)))
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			exchanges[0].Response.Body.Value = encoded
			transport, err := replay.NewHTTPTransport(exchanges)
			if err != nil {
				t.Fatal(err)
			}
			client, err := icloud.New(icloud.WithHTTPTransport(transport))
			if err != nil {
				t.Fatal(err)
			}
			request := icloud.RequestPCSAccessRequest{Auth: state.Auth, State: state, Service: protocol.AuthWebServicesPhotos}
			_, err = client.RequestPCSAccess(t.Context(), request)
			var failure *icloud.ClientError
			if !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse || string(failure.ResponseBody()) != body {
				t.Fatalf("malformed PCS response accepted or evidence lost: %v", err)
			}
			if err = transport.AssertConsumed(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNativeAuthWaitConfiguration(t *testing.T) {
	t.Parallel()

	_, err := icloud.New(icloud.WithAuthenticationWait(nil))

	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != icloud.Configuration {
		t.Fatalf("nil authentication wait accepted: %v", err)
	}
}

func nativeFixtureService(service *auth.AuthService) string {
	if service == nil || service.Url == nil {
		return ""
	}
	return *service.Url
}
