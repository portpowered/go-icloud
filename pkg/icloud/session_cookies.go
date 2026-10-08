package icloud

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

func bindSessionCookies(auth AuthContext, origin string) AuthContext {
	target, err := url.Parse(origin)
	if err != nil {
		return auth
	}

	for index := range auth.Cookies {
		cookie := &auth.Cookies[index]
		if cookie.Domain == "" {
			cookie.Domain, cookie.HostOnly = target.Hostname(), true
		}

		cookie.Domain = strings.ToLower(cookie.Domain)

		if cookie.Path == "" {
			cookie.Path = "/"
		}

		if cookie.MaxAge > 0 {
			expires := time.Now().Add(time.Duration(cookie.MaxAge) * time.Second)
			cookie.Expires, cookie.MaxAge = &expires, 0
		}
	}

	return auth
}

func mergeSessionCookie(cookies []AuthCookie, cookie AuthCookie) []AuthCookie {
	values := make([]AuthCookie, 0, len(cookies)+1)
	keep := cookie.MaxAge >= 0 && (cookie.Expires == nil || cookie.Expires.After(time.Now()))
	replaced := false

	for _, current := range cookies {
		if current.Name == cookie.Name && strings.ToLower(strings.TrimPrefix(current.Domain, ".")) == cookie.Domain &&
			current.Path == cookie.Path {
			if keep {
				values = append(values, cookie)
			}

			replaced = true

			continue
		}

		values = append(values, current)
	}

	if keep && !replaced {
		values = append(values, cookie)
	}

	return values
}

func applySessionResponse(auth AuthContext, response ResponseMetadata) AuthContext {
	target, err := url.Parse(response.CookieScopeURL)
	if err != nil || target.Hostname() == "" {
		return auth
	}

	reply := new(http.Response)

	reply.Header = requestHeaders(response.Headers)

	for _, cookie := range reply.Cookies() {
		value, valid := driveResponseCookie(cookie, target)
		if valid {
			auth.Cookies = mergeSessionCookie(auth.Cookies, value)
		}
	}

	return auth
}
