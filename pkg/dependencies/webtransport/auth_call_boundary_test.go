package webtransport_test

import (
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// Embedding a supported call satisfies the interface but does not add a supported operation.
type embeddedAuthenticationCall struct {
	webtransport.GetAuthChallengeCall
	Body []byte
}

func TestAuthenticationCallRejectsIncompatibleMediaBeforeTransport(t *testing.T) {
	wrong := "text/json"
	origin := "https://accounts.example.test"
	for name, testcase := range map[string]struct {
		call    webtransport.AuthenticationCall
		headers http.Header
	}{
		"typed-parameters": {
			call: webtransport.InitAuthSRPCall{Origin: origin,
				Params: &authapi.InitAuthSRPParams{ContentType: &wrong}, Body: auth.AuthSRPInitRequest{}},
		},
		"body-header": {call: webtransport.InitAuthSRPCall{Origin: origin},
			headers: http.Header{"Content-Type": {wrong}}},
		"logout-header": {call: webtransport.LogoutAuthSessionCall{Origin: origin},
			headers: http.Header{"Content-Type": {"application/json"}}},
		"second-header": {call: webtransport.InitAuthSRPCall{Origin: origin},
			headers: http.Header{"Content-Type": {"application/json", wrong}}},
		"lowercase-header": {call: webtransport.InitAuthSRPCall{Origin: origin},
			headers: http.Header{"content-type": {wrong}}},
		"mixedcase-header": {call: webtransport.InitAuthSRPCall{Origin: origin},
			headers: http.Header{"Content-Type": {"application/json"}, "CONTENT-TYPE": {wrong}}},
		"retrieval-header": {call: webtransport.GetAuthChallengeCall{Origin: origin},
			headers: http.Header{"Content-Type": {wrong}}},
	} {
		t.Run(name, func(t *testing.T) {
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
	for name, testcase := range map[string]struct {
		call    webtransport.AuthenticationCall
		headers http.Header
		media   string
	}{
		"json-default":     {call: webtransport.InitAuthSRPCall{Origin: "https://accounts.example.test"}, media: "application/json"},
		"logout-default":   {call: webtransport.LogoutAuthSessionCall{Origin: "https://accounts.example.test"}, media: "text/plain;charset=UTF-8"},
		"retrieval-absent": {call: webtransport.GetAuthChallengeCall{Origin: "https://accounts.example.test"}},
		"retrieval-json": {call: webtransport.GetAuthChallengeCall{Origin: "https://accounts.example.test"},
			headers: http.Header{"Content-Type": {"application/json", "application/json"}}, media: "application/json"},
		"mixedcase-json": {call: webtransport.InitAuthSRPCall{Origin: "https://accounts.example.test"},
			headers: http.Header{"content-type": {"application/json"}, "Content-Type": {"application/json"}}, media: "application/json"},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			client := webtransport.New(findMyRoundTrip(func(request *http.Request) (*http.Response, error) {
				calls++
				values := request.Header.Values("Content-Type")
				if request.Header.Get("Content-Type") != testcase.media || len(values) > 1 {
					t.Fatalf("actual sent media differs from the operation: %v", values)
				}
				for name := range request.Header {
					if http.CanonicalHeaderKey(name) == "Content-Type" && name != "Content-Type" {
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
	for name, call := range map[string]webtransport.AuthenticationCall{
		"nil":       nil,
		"typed-nil": (*webtransport.GetAuthChallengeCall)(nil),
		"embedded": embeddedAuthenticationCall{
			GetAuthChallengeCall: webtransport.GetAuthChallengeCall{Origin: "https://accounts.example.test"},
			Body:                 []byte("arbitrary body"),
		},
		"non-https": webtransport.GetAuthChallengeCall{Origin: "http://accounts.example.test"},
	} {
		t.Run(name, func(t *testing.T) {
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
