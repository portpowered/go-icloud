// Package savedlogin_test binds imported credentials to pinned Source local-file behavior.
package savedlogin_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/savedlogin"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const (
	syntheticClientID     = "synthetic-client"
	syntheticSessionToken = "synthetic-token"
)

type fixture struct {
	Cases []loginCase `json:"cases"`
}
type loginCase struct {
	ExpectedHeaders map[string]string   `json:"expectedHeaders"`
	Identity        json.RawMessage     `json:"identity"`
	AccountKey      string              `json:"accountKey"`
	Filename        string              `json:"filename"`
	Session         json.RawMessage     `json:"session"`
	CookieFile      string              `json:"cookieFile"`
	ExpectedCookies []icloud.AuthCookie `json:"expectedCookies"`
	SetupOrigin     string              `json:"setupOrigin"`
}

func TestSourceLocalCredentials(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../../../tests/replay/fixtures/synthetic/local/reference-logins.json")
	if err != nil {
		t.Fatal(err)
	}

	var corpus fixture

	err = json.Unmarshal(data, &corpus)
	if err != nil {
		t.Fatal(err)
	}

	for _, item := range corpus.Cases {
		t.Run(item.Filename, func(t *testing.T) {
			t.Parallel()
			verifyLogin(t, item)
		})
	}
}

func verifyLogin(t *testing.T, item loginCase) {
	t.Helper()
	root, files := prepareLogin(t, item)

	result, err := savedlogin.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	assertLogin(t, item, result, root)

	for path, value := range files {
		after, readErr := os.ReadFile(filepath.Clean(path))
		if readErr != nil {
			t.Fatal(readErr)
		}

		if string(after) != string(value) {
			t.Fatal("import modified reference files")
		}
	}
}

func prepareLogin(t *testing.T, item loginCase) (string, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	account := filepath.Join(root, "accounts", item.AccountKey)

	err := os.MkdirAll(account, 0o700)
	if err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{
		filepath.Join(root, "active-account.json"):         item.Identity,
		filepath.Join(account, item.Filename+".session"):   item.Session,
		filepath.Join(account, item.Filename+".cookiejar"): []byte(item.CookieFile),
	}
	for path, value := range files {
		err = os.WriteFile(path, value, 0o600)
		if err != nil {
			t.Fatal(err)
		}
	}

	return root, files
}

func assertLogin(t *testing.T, item loginCase, result *savedlogin.Login, root string) {
	t.Helper()

	actualHeaders := make(map[string]string)
	for _, header := range result.Request.Auth.Headers {
		actualHeaders[header.Name] = header.Value
	}

	if !reflect.DeepEqual(actualHeaders, item.ExpectedHeaders) {
		t.Fatal("reference constructor headers changed")
	}

	if !reflect.DeepEqual(result.Request.Auth.Cookies, item.ExpectedCookies) {
		t.Fatalf("imported synthetic cookies differ: %#v", result.Request.Auth.Cookies)
	}

	if result.Request.Auth.ClientID != syntheticClientID || result.Request.Auth.SessionToken != syntheticSessionToken ||
		result.Request.TrustToken != "synthetic-trust" || result.Request.Auth.SetupServiceURL != item.SetupOrigin ||
		result.Output != filepath.Join(root, "accounts", item.AccountKey, "go-session.json") {
		t.Fatal("reference identity or credentials changed")
	}
}
