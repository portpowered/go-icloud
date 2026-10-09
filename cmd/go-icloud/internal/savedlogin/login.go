// Package savedlogin imports the pinned reference CLI's private login files.
package savedlogin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/referenceconfig"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const accountKeyLength = 24

var errIdentity = errors.New("saved reference identity is incomplete")
var errCredentials = errors.New("saved reference credentials are incomplete")

// Login contains caller-owned credentials and a private native-session destination.
type Login struct {
	// Request contains credentials before service discovery.
	Request icloud.ResumeSessionRequest
	// Output is adjacent to this account's existing protected reference files.
	Output string
	// SourceFiles are imported reference files that credential export must preserve.
	SourceFiles []string
}

type identity struct {
	Username string `json:"username"`
	China    bool   `json:"china"`
}

//nolint:tagliatelle // The pinned reference's credential file fixes these spellings.
type sessionData struct {
	ClientID string                    `json:"client_id"`
	Token    string                    `json:"session_token"`
	Trust    string                    `json:"trust_token"`
	Country  nullable.Nullable[string] `json:"account_country"`
}

// Load reads the reference's active identity, session JSON and LWP cookie jar.
// It performs no network requests or updates to the reference files.
func Load(root string) (*Login, error) {
	var account identity

	err := readJSON(filepath.Join(root, "active-account.json"), &account)
	if err != nil {
		return nil, err
	}

	if account.Username == "" {
		return nil, errIdentity
	}

	digest := sha256.Sum256([]byte(account.Username))
	key := hex.EncodeToString(digest[:])[:accountKeyLength]
	directory := filepath.Join(root, "accounts", key)
	name := referenceFilename(account.Username)

	var session sessionData

	err = readJSON(filepath.Join(directory, name+".session"), &session)
	if err != nil {
		return nil, err
	}

	if session.ClientID == "" || session.Token == "" {
		return nil, errCredentials
	}

	cookies, err := readCookies(filepath.Join(directory, name+".cookiejar"))
	if err != nil {
		return nil, err
	}

	setup := string(referenceconfig.GlobalSetup)
	if account.China {
		setup = string(referenceconfig.ChinaSetup)
	}

	var request icloud.ResumeSessionRequest

	request.Auth = icloud.SavedSessionCredentials{ClientID: session.ClientID, SessionToken: session.Token,
		SetupServiceURL: setup, Headers: referenceHeaders(account.China), Cookies: cookies, ChinaMainland: &account.China}
	request.AccountCountryCode = session.Country
	request.TrustToken = session.Trust

	return &Login{Request: request, Output: filepath.Join(directory, "go-session.json"), SourceFiles: []string{
		filepath.Join(root, "active-account.json"), filepath.Join(directory, name+".session"),
		filepath.Join(directory, name+".cookiejar"),
	}}, nil
}

func readJSON(path string, result any) error {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("read reference login file: %w", err)
	}

	err = json.Unmarshal(data, result)
	if err != nil {
		return fmt.Errorf("decode reference login file: %w", err)
	}

	return nil
}

func referenceFilename(username string) string {
	return strings.Map(func(value rune) rune {
		if unicode.IsLetter(value) || unicode.IsNumber(value) || value == '_' {
			return value
		}

		return -1
	}, username)
}
