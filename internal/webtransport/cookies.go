package webtransport

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"

	"github.com/portpowered/go-icloud/internal/protocol"
)

// CookieSeed preserves whether a supplied host binds exactly or includes subdomains.
type CookieSeed struct {
	Cookie   *http.Cookie
	HostOnly bool
}

// CookieState owns a fresh operation-local jar and copied initial credentials.
// It is never stored on the reusable HTTP client.
type CookieState struct {
	jar   http.CookieJar
	seeds []CookieSeed
	once  sync.Once
}

// NewCookieState copies initial cookies without changing caller state.
func NewCookieState(cookies []CookieSeed) (*CookieState, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create account cookie state: %w", err)
	}

	seeds := make([]CookieSeed, 0, len(cookies))

	for _, cookie := range cookies {
		//nolint:gosec // LIB-05: preserve supplied native cookie attributes rather than inventing browser policy.
		copyCookie := *cookie.Cookie
		copyCookie.Unparsed = append([]string(nil), cookie.Cookie.Unparsed...)
		seeds = append(seeds, CookieSeed{Cookie: &copyCookie, HostOnly: cookie.HostOnly})
	}

	return &CookieState{jar: jar, seeds: seeds, once: sync.Once{}}, nil
}

func (state *CookieState) initialize(target *url.URL) {
	state.once.Do(func() {
		for _, cookie := range state.seeds {
			origin := *target
			if cookie.Cookie.Domain != "" {
				origin.Host = strings.TrimPrefix(cookie.Cookie.Domain, ".")
			}

			//nolint:gosec // LIB-05: copy captured cookie flags exactly for native requests.
			value := *cookie.Cookie
			if cookie.HostOnly {
				value.Domain = ""
			}

			state.jar.SetCookies(&origin, []*http.Cookie{&value})
		}
	})
}

func (state *CookieState) apply(request *http.Request) {
	if state == nil {
		return
	}

	state.initialize(request.URL)

	// An explicit session Cookie header takes precedence in the reference too.
	if request.Header.Get(protocol.CookieName) != "" {
		return
	}

	cookies := state.jar.Cookies(request.URL)
	pairs := make([]string, 0, len(cookies))

	for _, cookie := range cookies {
		pairs = append(pairs, fmt.Sprintf(protocol.CookiePairFormatValue, cookie.Name, NativeCookieValue(cookie)))
	}

	if len(pairs) != 0 {
		request.Header.Set(protocol.CookieName, strings.Join(pairs, protocol.CookieSeparatorValue))
	}
}

func (state *CookieState) update(target *url.URL, response *http.Response) {
	if state == nil {
		return
	}

	state.initialize(target)
	state.jar.SetCookies(target, response.Cookies())
}

// NativeCookieValue preserves version-zero cookie framing used by the reference.
func NativeCookieValue(cookie *http.Cookie) string {
	if cookie.Quoted {
		return fmt.Sprintf(protocol.CookieQuotedValueFormatValue, cookie.Value)
	}

	return cookie.Value
}
