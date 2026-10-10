package webtransport_test

import (
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
)

const (
	cookieBoundaryDomain     = ".example.test"
	cookieBoundaryExcluded   = "excluded"
	cookieBoundaryContentURL = "https://cdn.example.test/allowed/a%2Fb?sig=first&sig=second"
)

type cookieBoundaryCase struct {
	name           string
	authentication bool
	explicit       bool
}

// Synthetic controls cover URI/cookie scope at the real content and auth send
// boundaries; these values are synthetic and contain no account credentials.
func TestSessionCookieContentAndAuthenticationScope(t *testing.T) {
	t.Parallel()

	cases := []cookieBoundaryCase{
		{name: "content-jar", authentication: false, explicit: false},
		{name: "content-explicit", authentication: false, explicit: true},
		{name: "authentication-jar", authentication: true, explicit: false},
		{name: "authentication-explicit", authentication: true, explicit: true},
	}
	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			runCookieBoundaryCase(t, testcase)
		})
	}
}

func runCookieBoundaryCase(t *testing.T, testcase cookieBoundaryCase) {
	t.Helper()

	state, seeds := cookieBoundaryState(t, testcase.authentication)

	headers := make(http.Header)
	if testcase.explicit {
		headers.Set(protocol.CookieName, "explicit=owned")
	}

	calls := 0
	client := webtransport.New(findMyRoundTrip(func(request *http.Request) (*http.Response, error) {
		calls++

		checkCookieBoundaryRequest(t, testcase, request)

		return findMyTestResponse(`{}`), nil
	}))

	var err error

	if testcase.authentication {
		request, requestErr := authapi.NewGetAuthChallengeRequest("https://accounts.example.test", nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}

		_, err = client.ExchangeAuthentication(t.Context(), request, headers, state)
	} else {
		auth := findMyTestAuth()
		auth.Headers, auth.Cookies = headers, state
		_, err = client.DownloadPhotoContent(t.Context(), auth, cookieBoundaryContentURL)
	}

	if err != nil || calls != 1 {
		t.Fatalf("cookie boundary failed: calls=%d error=%v", calls, err)
	}

	if seeds[0].Cookie.Value != "owned" || seeds[0].Cookie.Domain != cookieBoundaryDomain {
		t.Fatal("caller cookies were changed")
	}
}

func cookieBoundaryState(t *testing.T, authentication bool) (*webtransport.CookieState, []webtransport.CookieSeed) {
	t.Helper()

	cookiePath := "/allowed"
	if authentication {
		cookiePath = "/appleauth"
	}

	seeds := []webtransport.CookieSeed{
		{Cookie: syntheticBoundaryCookie("session", "owned", cookieBoundaryDomain, cookiePath), HostOnly: false},
		{Cookie: syntheticBoundaryCookie("foreign", cookieBoundaryExcluded, "other.invalid", "/"), HostOnly: false},
		{Cookie: syntheticBoundaryCookie("wrongpath", cookieBoundaryExcluded,
			cookieBoundaryDomain, "/different"), HostOnly: false},
	}

	state, err := webtransport.NewCookieState(seeds)
	if err != nil {
		t.Fatal(err)
	}

	return state, seeds
}

func syntheticBoundaryCookie(name, value, domain, path string) *http.Cookie {
	cookie := new(http.Cookie)
	cookie.Name, cookie.Value, cookie.Domain, cookie.Path = name, value, domain, path
	cookie.Secure, cookie.HttpOnly, cookie.SameSite = true, true, http.SameSiteStrictMode

	return cookie
}

func checkCookieBoundaryRequest(t *testing.T, testcase cookieBoundaryCase, request *http.Request) {
	t.Helper()

	expected := "session=owned"
	if testcase.explicit {
		expected = "explicit=owned"
	}

	if request.Header.Get(protocol.CookieName) != expected {
		t.Fatal("cookie scope or explicit-header precedence changed")
	}

	if !testcase.authentication && (request.URL.EscapedPath() != "/allowed/a%2Fb" ||
		request.URL.RawQuery != "sig=first&sig=second") {
		t.Fatal("content locator changed")
	}
}
