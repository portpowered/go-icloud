package icloud

import (
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"net/http"
)

func authCookies(cookies []AuthCookie) []webtransport.CookieSeed {
	result := make([]webtransport.CookieSeed, 0, len(cookies))

	for _, source := range cookies {
		//nolint:gosec // API-03: copy caller cookie attributes; forcing browser flags changes native protocol behavior.
		cookie := new(http.Cookie)
		cookie.Name, cookie.Value = source.Name, source.Value
		cookie.Domain, cookie.Path = source.Domain, source.Path
		cookie.Secure, cookie.HttpOnly, cookie.MaxAge = source.Secure, source.HTTPOnly, source.MaxAge

		cookie.SameSite = authCookieSameSite(source.SameSite)

		if source.Expires != nil {
			cookie.Expires = *source.Expires
		}

		result = append(result, webtransport.CookieSeed{Cookie: cookie, HostOnly: source.HostOnly})
	}

	return result
}

func authCookieSameSite(value *AuthCookieSameSite) http.SameSite {
	if value == nil {
		return http.SameSiteDefaultMode
	}

	switch *value {
	case AuthCookieSameSiteLax:
		return http.SameSiteLaxMode
	case AuthCookieSameSiteStrict:
		return http.SameSiteStrictMode
	case AuthCookieSameSiteNone:
		return http.SameSiteNoneMode
	case AuthCookieSameSiteDefault:
		return http.SameSiteDefaultMode
	default:
		return http.SameSiteDefaultMode
	}
}
