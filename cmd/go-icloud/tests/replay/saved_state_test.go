package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type referenceSavedCookie struct {
	Name            string `json:"name"`
	Value           string `json:"value"`
	Domain          string `json:"domain"`
	Path            string `json:"path"`
	Secure          bool   `json:"secure"`
	DomainSpecified bool   `json:"domainSpecified"`
	Expires         *int64 `json:"expires"`
	HTTPOnly        bool   `json:"httpOnly"`
}

func assertCompleteReferenceSession(t *testing.T, row map[string]json.RawMessage,
	logins []map[string]json.RawMessage, path, stateKey string, exchange replay.Exchange,
) {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var actual icloud.ResumeSessionResult

	decode(t, data, &actual)
	expected := expectedReferenceSession(t, row, stateKey, exchange)
	expected.Auth.ChinaMainland = referenceRegion(t, row, logins)
	assertSavedSessionSnapshot(t, actual, expected)
}

func assertSavedSessionSnapshot(t *testing.T, actual, expected icloud.ResumeSessionResult) {
	t.Helper()
	actual.AccountData = compactSavedAccount(t, actual.AccountData)
	expected.AccountData = compactSavedAccount(t, expected.AccountData)

	sortSavedHeaders(actual.Auth.Headers)
	sortSavedHeaders(expected.Auth.Headers)
	normalizeSavedDomains(actual.Auth.Cookies)
	normalizeSavedDomains(expected.Auth.Cookies)

	if !reflect.DeepEqual(actual, expected) {
		t.Logf("response counts: %d and %d", len(actual.Responses), len(expected.Responses))

		for index, response := range actual.Responses {
			if index < len(expected.Responses) {
				logSavedStateDifferences(t, reflect.ValueOf(response), reflect.ValueOf(expected.Responses[index]))
			}
		}

		logSavedStateDifferences(t, reflect.ValueOf(actual), reflect.ValueOf(expected))
		logSavedStateDifferences(t, reflect.ValueOf(actual.Auth), reflect.ValueOf(expected.Auth))

		for index, cookie := range actual.Auth.Cookies {
			if index < len(expected.Auth.Cookies) {
				logSavedStateDifferences(t, reflect.ValueOf(cookie), reflect.ValueOf(expected.Auth.Cookies[index]))
			}
		}

		t.Fatal("complete persisted session differs from Source state and response metadata")
	}
}

func normalizeSavedDomains(cookies []icloud.AuthCookie) {
	for index := range cookies {
		// Domain cookies have equivalent DNS scope with or without a leading dot.
		// HostOnly remains independently compared; no cookie attribute is omitted.
		cookies[index].Domain = strings.ToLower(strings.TrimPrefix(cookies[index].Domain, "."))
	}
}

func compactSavedAccount(t *testing.T, value json.RawMessage) json.RawMessage {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	return encoded
}

func logSavedStateDifferences(t *testing.T, actual, expected reflect.Value) {
	t.Helper()

	for index := range actual.NumField() {
		if !reflect.DeepEqual(actual.Field(index).Interface(), expected.Field(index).Interface()) {
			t.Logf("saved %s.%s differs", actual.Type().Name(), actual.Type().Field(index).Name)
		}
	}
}

func expectedReferenceSession(t *testing.T, row map[string]json.RawMessage,
	stateKey string, exchange replay.Exchange,
) icloud.ResumeSessionResult {
	t.Helper()

	var (
		expected       icloud.ResumeSessionResult
		state, session map[string]json.RawMessage
		params         map[string]string
		services       map[string]map[string]string
	)

	decode(t, row[stateKey], &state)
	decode(t, state["session_data"], &session)
	decode(t, state["params"], &params)
	decode(t, state["webservices"], &services)
	decode(t, session["session_token"], &expected.Auth.SessionToken)
	decode(t, session["trust_token"], &expected.TrustToken)
	decode(t, session["account_country"], &expected.AccountCountryCode)
	decode(t, state["trusted_session"], &expected.TrustedSession)
	decode(t, state["requires_2fa"], &expected.RequiresTwoFactor)
	decode(t, state["requires_2sa"], &expected.RequiresTwoStep)

	build, mastering := params["clientBuildNumber"], params["clientMasteringNumber"]
	expected.Auth.ClientBuildNumber, expected.Auth.ClientMasteringNumber = &build, &mastering
	expected.Auth.ClientID, expected.Auth.AccountID = params["clientId"], params["dsid"]
	expected.Auth.SetupServiceURL = exchange.Request.Origin
	expected.Auth.AccountServiceURL = services["account"]["url"]
	expected.Auth.DriveServiceURL = services["drivews"]["url"]
	expected.Auth.DriveDocumentServiceURL = services["docws"]["url"]
	expected.Auth.FindMyServiceURL = services["findme"]["url"]
	expected.Auth.Headers = referenceSavedHeaders(t, row)
	expected.Auth.Cookies = referenceSavedCookies(t, row)
	expected.AccountData = referenceResponseBody(t, exchange.Response.Body)
	expected.Responses = []icloud.ResponseMetadata{referenceResponseMetadata(exchange)}

	return expected
}

func referenceSavedHeaders(t *testing.T, row map[string]json.RawMessage) []icloud.Header {
	t.Helper()

	var values map[string]string

	decode(t, row["headers"], &values)

	headers := make([]icloud.Header, 0, len(values))
	for name, value := range values {
		headers = append(headers, icloud.Header{Name: name, Value: value})
	}

	return headers
}

func referenceSavedCookies(t *testing.T, row map[string]json.RawMessage) []icloud.AuthCookie {
	t.Helper()

	var values []referenceSavedCookie

	decode(t, row["cookieState"], &values)

	cookies := make([]icloud.AuthCookie, 0, len(values))

	for _, value := range values {
		var cookie icloud.AuthCookie

		cookie.Name, cookie.Value = value.Name, value.Value
		cookie.Domain, cookie.Path = value.Domain, value.Path
		cookie.HostOnly = !value.DomainSpecified

		cookie.Secure, cookie.HTTPOnly = value.Secure, value.HTTPOnly

		if value.Expires != nil {
			expires := time.Unix(*value.Expires, 0).UTC()
			cookie.Expires = &expires
		}

		cookies = append(cookies, cookie)
	}

	return cookies
}

func referenceRegion(t *testing.T, row map[string]json.RawMessage,
	logins []map[string]json.RawMessage,
) *bool {
	t.Helper()

	var expected string

	decode(t, row["localCase"], &expected)

	for _, login := range logins {
		var filename string

		decode(t, login["filename"], &filename)

		if filename != expected {
			continue
		}

		var (
			identity map[string]json.RawMessage
			region   *bool
		)

		decode(t, login["identity"], &identity)
		decode(t, identity["china"], &region)

		return region
	}

	t.Fatal("reference region not found")

	return nil
}

func referenceResponseBody(t *testing.T, body replay.Entity) []byte {
	t.Helper()

	var encoded string

	decode(t, body.Value, &encoded)

	result, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func referenceResponseMetadata(exchange replay.Exchange) icloud.ResponseMetadata {
	var result icloud.ResponseMetadata

	result.StatusCode = exchange.Response.Status

	result.CookieScopeURL = exchange.Request.Origin + exchange.Request.Path
	for _, header := range exchange.Response.Headers {
		result.Headers = append(result.Headers, icloud.Header{Name: http.CanonicalHeaderKey(header[0]), Value: header[1]})
	}

	sortSavedHeaders(result.Headers)

	return result
}

func sortSavedHeaders(headers []icloud.Header) {
	slices.SortStableFunc(headers, func(first, second icloud.Header) int {
		return strings.Compare(first.Name, second.Name)
	})
}
