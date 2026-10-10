package webtransport_test

import (
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
)

// Synthetic controls cover exact URI/cookie scope at the real content and
// authentication send boundaries; they contain no captured account credentials.
func TestSessionCookieContentAndAuthenticationScope(t *testing.T) {
	t.Parallel()
	for _, authentication := range []bool{false, true} {
		for _, explicit := range []bool{false, true} {
			t.Run(map[bool]string{false: "content", true: "authentication"}[authentication]+map[bool]string{false: "-jar", true: "-explicit"}[explicit], func(t *testing.T) {
				t.Parallel()
				cookiePath := "/allowed"
				if authentication {
					cookiePath = "/appleauth"
				}
				seeds := []webtransport.CookieSeed{
					{Cookie: &http.Cookie{Name: "session", Value: "owned", Domain: ".example.test", Path: cookiePath, Secure: true}, HostOnly: false},
					{Cookie: &http.Cookie{Name: "foreign", Value: "excluded", Domain: "other.invalid", Path: "/", Secure: true}, HostOnly: false},
					{Cookie: &http.Cookie{Name: "wrongpath", Value: "excluded", Domain: ".example.test", Path: "/different", Secure: true}, HostOnly: false},
				}
				state, err := webtransport.NewCookieState(seeds)
				if err != nil {
					t.Fatal(err)
				}
				headers := make(http.Header)
				if explicit {
					headers.Set(protocol.CookieName, "explicit=owned")
				}
				calls := 0
				client := webtransport.New(findMyRoundTrip(func(request *http.Request) (*http.Response, error) {
					calls++
					expected := "session=owned"
					if explicit {
						expected = "explicit=owned"
					}
					if request.Header.Get(protocol.CookieName) != expected {
						t.Fatal("cookie scope or explicit-header precedence changed")
					}
					if !authentication && (request.URL.EscapedPath() != "/allowed/a%2Fb" || request.URL.RawQuery != "sig=first&sig=second") {
						t.Fatal("content locator changed")
					}
					return findMyTestResponse(`{}`), nil
				}))
				if authentication {
					request, requestErr := authapi.NewGetAuthChallengeRequest("https://accounts.example.test", nil)
					if requestErr != nil {
						t.Fatal(requestErr)
					}
					_, err = client.ExchangeAuthentication(t.Context(), request, headers, state)
				} else {
					_, err = client.DownloadPhotoContent(t.Context(), webtransport.RequestContext{Headers: headers, Cookies: state}, "https://cdn.example.test/allowed/a%2Fb?sig=first&sig=second")
				}
				if err != nil || calls != 1 {
					t.Fatalf("cookie boundary failed: calls=%d error=%v", calls, err)
				}
				if seeds[0].Cookie.Value != "owned" || seeds[0].Cookie.Domain != ".example.test" {
					t.Fatal("caller cookies were changed")
				}
			})
		}
	}
}
