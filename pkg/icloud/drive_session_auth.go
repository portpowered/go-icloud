package icloud

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
)

func (session *DriveSession) bindInitialCookies() {
	session.auth = bindSessionCookies(session.auth, session.auth.DriveServiceURL)
}

func (session *DriveSession) setToken(token string) {
	if token == "" {
		return
	}

	session.mu.Lock()
	session.auth.DriveToken = token
	session.mu.Unlock()
}

func (session *DriveSession) observeFailure(err error) {
	var failure *ClientError
	if !errors.As(err, &failure) {
		return
	}

	session.setToken(failure.UploadToken())

	metadata := failure.PriorResponses()
	if failure.StatusCode() != 0 {
		metadata = append(metadata, ResponseMetadata{CookieScopeURL: failure.CookieScopeURL(),
			StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders()})
	}

	if len(metadata) != 0 {
		session.observe(metadata...)
	}
}

func (session *DriveSession) observe(metadata ...ResponseMetadata) {
	session.mu.Lock()
	session.responses = cloneDriveResponses(metadata)
	session.mu.Unlock()

	for _, response := range metadata {
		target, err := url.Parse(response.CookieScopeURL)
		if err != nil || target.Hostname() == "" {
			continue
		}

		reply := new(http.Response)

		reply.Header = requestHeaders(response.Headers)

		for _, cookie := range reply.Cookies() {
			value, valid := driveResponseCookie(cookie, target)
			if valid {
				session.storeCookie(value)
			}
		}
	}
}

func driveResponseCookie(cookie *http.Cookie, target *url.URL) (AuthCookie, bool) {
	domain := strings.TrimPrefix(strings.ToLower(cookie.Domain), ".")

	host := strings.ToLower(target.Hostname())

	if domain != "" && domain != host && !strings.HasSuffix(host, "."+domain) {
		var empty AuthCookie

		return empty, false
	}

	hostOnly := domain == ""
	if hostOnly {
		domain = host
	}

	path := cookie.Path
	if !strings.HasPrefix(path, "/") {
		path = driveDefaultCookiePath(target.Path)
	}

	value := AuthCookie{Name: cookie.Name, Value: webtransport.NativeCookieValue(cookie), Domain: domain, Path: path,
		HostOnly: hostOnly, Secure: cookie.Secure, HTTPOnly: cookie.HttpOnly, MaxAge: cookie.MaxAge,
		Expires: nil, SameSite: driveCookieSameSite(cookie.SameSite)}
	if !cookie.Expires.IsZero() {
		value.Expires = &cookie.Expires
	}

	if cookie.MaxAge > 0 {
		expires := time.Now().Add(time.Duration(cookie.MaxAge) * time.Second)
		value.Expires, value.MaxAge = &expires, 0
	}

	return value, true
}

func driveDefaultCookiePath(path string) string {
	index := strings.LastIndex(path, "/")
	if index <= 0 {
		return "/"
	}

	return path[:index]
}

func (session *DriveSession) storeCookie(cookie AuthCookie) {
	session.mu.Lock()
	defer session.mu.Unlock()

	session.auth.Cookies = mergeSessionCookie(session.auth.Cookies, cookie)
}

func driveCookieSameSite(mode http.SameSite) *AuthCookieSameSite {
	var value AuthCookieSameSite

	switch mode {
	case http.SameSiteLaxMode:
		value = AuthCookieSameSiteLax
	case http.SameSiteStrictMode:
		value = AuthCookieSameSiteStrict
	case http.SameSiteNoneMode:
		value = AuthCookieSameSiteNone
	case http.SameSiteDefaultMode:
		value = AuthCookieSameSiteDefault
	default:
		return nil
	}

	return &value
}
