package webtransport_test

import (
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

const (
	authBoundaryOrigin               string = "https://accounts.example.test"
	authBoundaryJSONMedia            string = "application/json"
	authBoundaryLogoutMedia          string = "text/plain;charset=UTF-8"
	authBoundaryMediaHeader          string = "Content-Type"
	authBoundaryLowercaseMediaHeader string = "content-type"
)

// Embedding a supported call satisfies the interface but does not add a supported operation.
type embeddedAuthenticationCall struct {
	webtransport.GetAuthChallengeCall

	Body []byte
}

func TestAuthenticationCallRejectsIncompatibleMediaBeforeTransport(t *testing.T) {
	t.Parallel()

	srpBody := auth.AuthSRPInitRequest{A: nil, AccountName: "", Protocols: nil}
	logoutBody := auth.AuthLogoutRequest{TrustBrowser: false, AllBrowsers: false}

	wrong := "text/json"
	params := new(authapi.InitAuthSRPParams)
	params.ContentType = &wrong

	for name, testcase := range map[string]struct {
		call    webtransport.AuthenticationCall
		headers http.Header
	}{
		"typed-parameters": {
			call: webtransport.InitAuthSRPCall{Origin: authBoundaryOrigin,
				Params: params, Body: srpBody},
			headers: nil,
		},
		"body-header": {call: webtransport.InitAuthSRPCall{Origin: authBoundaryOrigin, Params: nil, Body: srpBody},
			headers: http.Header{authBoundaryMediaHeader: {wrong}}},
		"logout-header": {call: webtransport.LogoutAuthSessionCall{Origin: authBoundaryOrigin, Params: nil, Body: logoutBody},
			headers: http.Header{authBoundaryMediaHeader: {authBoundaryJSONMedia}}},
		"second-header": {call: webtransport.InitAuthSRPCall{Origin: authBoundaryOrigin, Params: nil, Body: srpBody},
			headers: http.Header{authBoundaryMediaHeader: {authBoundaryJSONMedia, wrong}}},
		"lowercase-header": {call: webtransport.InitAuthSRPCall{Origin: authBoundaryOrigin, Params: nil, Body: srpBody},
			headers: http.Header{authBoundaryLowercaseMediaHeader: {wrong}}},
		"mixedcase-header": {call: webtransport.InitAuthSRPCall{Origin: authBoundaryOrigin, Params: nil, Body: srpBody},
			headers: http.Header{authBoundaryMediaHeader: {authBoundaryJSONMedia}, "CONTENT-TYPE": {wrong}}},
		"retrieval-header": {call: webtransport.GetAuthChallengeCall{Origin: authBoundaryOrigin, Params: nil},
			headers: http.Header{authBoundaryMediaHeader: {wrong}}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			calls := 0

			client := webtransport.New(findMyRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++

				return findMyTestResponse(`{}`), nil
			}))

			response, err := client.ExchangeAuthentication(t.Context(), testcase.call, testcase.headers, nil)
			if err == nil || response != nil || calls != 0 {
				t.Fatalf("incompatible authentication media reached transport: response=%v err=%v calls=%d", response, err, calls)
			}
		})
	}
}

func TestAuthenticationCallEmitsItsCanonicalMedia(t *testing.T) {
	t.Parallel()

	srpBody := auth.AuthSRPInitRequest{A: nil, AccountName: "", Protocols: nil}
	logoutBody := auth.AuthLogoutRequest{TrustBrowser: false, AllBrowsers: false}

	for name, testcase := range map[string]struct {
		call    webtransport.AuthenticationCall
		headers http.Header
		media   string
	}{
		"json-default": {call: webtransport.InitAuthSRPCall{Origin: authBoundaryOrigin, Params: nil, Body: srpBody},
			headers: nil, media: authBoundaryJSONMedia},
		"logout-default": {call: webtransport.LogoutAuthSessionCall{
			Origin: authBoundaryOrigin, Params: nil, Body: logoutBody,
		},
			headers: nil, media: authBoundaryLogoutMedia},
		"retrieval-absent": {call: webtransport.GetAuthChallengeCall{Origin: authBoundaryOrigin, Params: nil},
			headers: nil, media: ""},
		"retrieval-json": {call: webtransport.GetAuthChallengeCall{Origin: authBoundaryOrigin, Params: nil},
			headers: http.Header{authBoundaryMediaHeader: {authBoundaryJSONMedia, authBoundaryJSONMedia}},
			media:   authBoundaryJSONMedia},
		"mixedcase-json": {call: webtransport.InitAuthSRPCall{Origin: authBoundaryOrigin, Params: nil, Body: srpBody},
			headers: http.Header{authBoundaryLowercaseMediaHeader: {authBoundaryJSONMedia},
				authBoundaryMediaHeader: {authBoundaryJSONMedia}},
			media: authBoundaryJSONMedia},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			calls := 0

			client := webtransport.New(findMyRoundTrip(func(request *http.Request) (*http.Response, error) {
				calls++

				values := request.Header.Values(authBoundaryMediaHeader)
				if request.Header.Get(authBoundaryMediaHeader) != testcase.media || len(values) > 1 {
					t.Fatalf("actual sent media differs from the operation: %v", values)
				}

				for name := range request.Header {
					if http.CanonicalHeaderKey(name) == authBoundaryMediaHeader &&
						name != authBoundaryMediaHeader {
						t.Fatalf("case-variant media survived normalization: %s", name)
					}
				}

				return findMyTestResponse(`{}`), nil
			}))

			_, err := client.ExchangeAuthentication(t.Context(), testcase.call, testcase.headers, nil)
			if err != nil || calls != 1 {
				t.Fatalf("canonical operation failed: err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestAuthenticationCallRejectsUnknownDynamicTypesBeforeTransport(t *testing.T) {
	t.Parallel()

	for name, call := range map[string]webtransport.AuthenticationCall{
		"nil":       nil,
		"typed-nil": (*webtransport.GetAuthChallengeCall)(nil),
		"embedded": embeddedAuthenticationCall{
			GetAuthChallengeCall: webtransport.GetAuthChallengeCall{
				Origin: authBoundaryOrigin, Params: nil,
			},
			Body: []byte("arbitrary body"),
		},
		"non-https":       webtransport.GetAuthChallengeCall{Origin: "http://accounts.example.test", Params: nil},
		"origin-path":     webtransport.GetAuthChallengeCall{Origin: "https://accounts.example.test/extra", Params: nil},
		"origin-query":    webtransport.GetAuthChallengeCall{Origin: "https://accounts.example.test?extra=value", Params: nil},
		"origin-fragment": webtransport.GetAuthChallengeCall{Origin: "https://accounts.example.test#extra", Params: nil},
		"origin-userinfo": webtransport.GetAuthChallengeCall{Origin: "https://user@accounts.example.test", Params: nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			calls := 0

			client := webtransport.New(findMyRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++

				return findMyTestResponse(`{}`), nil
			}))

			response, err := client.ExchangeAuthentication(t.Context(), call, nil, nil)
			if err == nil || response != nil || calls != 0 {
				t.Fatalf("invalid authentication call reached transport: response=%v err=%v calls=%d", response, err, calls)
			}
		})
	}
}
