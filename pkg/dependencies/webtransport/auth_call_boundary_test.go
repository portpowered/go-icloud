package webtransport_test

import (
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
)

// Embedding a supported call satisfies the interface but does not add a supported operation.
type embeddedAuthenticationCall struct {
	webtransport.GetAuthChallengeCall
	Body []byte
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
