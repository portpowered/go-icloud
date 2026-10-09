package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestReferenceImportResumeAndRead(t *testing.T) {
	t.Parallel()
	corpus := readObject(t, "../../../../tests/replay/fixtures/synthetic/local/reference-resume-repeat.json")
	local := readObject(t, "../../../../tests/replay/fixtures/synthetic/local/reference-logins.json")

	var cases, logins []map[string]json.RawMessage

	decode(t, corpus["cases"], &cases)
	decode(t, local["cases"], &logins)

	if len(cases) != 4 {
		t.Fatal("reference CLI flow inventory changed")
	}

	for _, row := range cases {
		var name string

		decode(t, row["name"], &name)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runReferenceResumeRead(t, row, logins)
		})
	}
}

// This CLI filesystem control uses the Source authentication pair, then refuses
// to replace an existing directory with private credentials.
func TestReferenceResumeSaveFailure(t *testing.T) {
	t.Parallel()
	corpus := readObject(t, "../../../../tests/replay/fixtures/synthetic/local/reference-resume.json")
	local := readObject(t, "../../../../tests/replay/fixtures/synthetic/local/reference-logins.json")

	var (
		cases, logins []map[string]json.RawMessage
		exchanges     []replay.Exchange
	)

	decode(t, corpus["cases"], &cases)
	decode(t, local["cases"], &logins)
	decode(t, cases[0]["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges[:1])
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	root, _, files := writeReferenceLogin(t, cases[0], logins)
	destination := filepath.Join(t.TempDir(), "existing")

	err = os.Mkdir(destination, 0700)
	if err != nil {
		t.Fatal(err)
	}

	var output, diagnostic bytes.Buffer

	err = command.Run(t.Context(), client,
		[]string{referenceStateFlag, root, saveSessionFlag, destination, resumeCommand}, &output, &diagnostic)
	assertReferenceSaveFailure(t, err, destination, output.Bytes())

	for path, data := range files {
		assertResumeInputUnchanged(t, path, data, diagnostic.Bytes())
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

// This local credential-export control requires no provider request.
func TestReferenceResumePreservesImportedFiles(t *testing.T) {
	t.Parallel()
	corpus := readObject(t, "../../../../tests/replay/fixtures/synthetic/local/reference-resume.json")
	local := readObject(t, "../../../../tests/replay/fixtures/synthetic/local/reference-logins.json")

	var cases, logins []map[string]json.RawMessage

	decode(t, corpus["cases"], &cases)
	decode(t, local["cases"], &logins)

	root, _, files := writeReferenceLogin(t, cases[0], logins)
	for source := range files {
		assertReferenceOverwriteRefused(t, root, source, files)

		alias := filepath.Join(t.TempDir(), "alias.json")

		err := os.Link(source, alias)
		if err != nil {
			t.Fatal(err)
		}

		assertReferenceOverwriteRefused(t, root, alias, files)
	}
}

func assertReferenceOverwriteRefused(t *testing.T, root, destination string, files map[string][]byte) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(nil)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	var (
		output, diagnostic bytes.Buffer
		failure            *command.SessionError
	)

	err = command.Run(t.Context(), client,
		[]string{referenceStateFlag, root, saveSessionFlag, destination, resumeCommand}, &output, &diagnostic)
	if !errors.As(err, &failure) || len(output.Bytes()) != 0 ||
		failure.Cause.Error() != "native session destination is an imported reference file" {
		t.Fatal("reference overwrite was not refused before authentication")
	}

	for path, data := range files {
		assertResumeInputUnchanged(t, path, data, diagnostic.Bytes())
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func assertReferenceSaveFailure(t *testing.T, err error, destination string, output []byte) {
	t.Helper()

	var (
		failure *command.SessionError
		cause   *os.LinkError
	)

	if !errors.As(err, &failure) || !errors.As(err, &cause) || len(output) != 0 {
		t.Fatal("credential save failure lost its cause or printed session state")
	}

	if failure.Error() == "" || !errors.Is(errors.Unwrap(failure), failure.Cause) {
		t.Fatal("credential save failure changed its display or cause")
	}

	assertReferenceSaveCleanup(t, destination)
}

func assertReferenceSaveCleanup(t *testing.T, destination string) {
	t.Helper()

	info, statErr := os.Stat(destination)
	if statErr != nil || !info.IsDir() {
		t.Fatal("credential save failure replaced the existing directory")
	}

	entries, readErr := os.ReadDir(filepath.Dir(destination))
	if readErr != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(destination) {
		t.Fatal("credential save failure left private temporary files")
	}
}

func writeReferenceLogin(t *testing.T, row map[string]json.RawMessage,
	logins []map[string]json.RawMessage,
) (string, string, map[string][]byte) {
	t.Helper()

	var expected string

	decode(t, row["localCase"], &expected)

	for _, login := range logins {
		var filename, key, cookies string

		decode(t, login["filename"], &filename)

		if filename != expected {
			continue
		}

		decode(t, login["accountKey"], &key)
		decode(t, login["cookieFile"], &cookies)
		root := t.TempDir()

		directory := filepath.Join(root, "accounts", key)

		err := os.MkdirAll(directory, 0700)
		if err != nil {
			t.Fatal(err)
		}

		files := map[string][]byte{
			filepath.Join(root, "active-account.json"):      login["identity"],
			filepath.Join(directory, filename+".session"):   login["session"],
			filepath.Join(directory, filename+".cookiejar"): []byte(cookies),
		}
		for path, data := range files {
			err := os.WriteFile(path, data, 0600)
			if err != nil {
				t.Fatal(err)
			}
		}

		return root, filepath.Join(directory, "go-session.json"), files
	}

	t.Fatal("reference login not found")

	return "", "", nil
}

func runReferenceResumeRead(t *testing.T, row map[string]json.RawMessage,
	logins []map[string]json.RawMessage,
) {
	t.Helper()

	var exchanges []replay.Exchange

	decode(t, row["exchanges"], &exchanges)

	transport, err := replay.NewHTTPTransport(exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	root, destination, files := writeReferenceLogin(t, row, logins)

	var output, diagnostic bytes.Buffer

	err = command.Run(t.Context(), client, []string{referenceStateFlag, root, resumeCommand}, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := json.Marshal(map[string]json.RawMessage{"auth_state": row["authState"]})
	if err != nil {
		t.Fatal(err)
	}

	assertNativeSaved(t, map[string]json.RawMessage{"result": expected}, destination, output.Bytes())
	assertCompleteReferenceSession(t, row, logins, destination, "authState", exchanges[0])
	output.Reset()

	err = command.Run(t.Context(), client, []string{sessionFlag, destination, accountDevicesCommand}, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}

	assertReferenceDeviceResults(t, row, output.Bytes())

	assertSecondReferenceResume(t, client, row, logins, destination, exchanges[2])

	for path, data := range files {
		assertResumeInputUnchanged(t, path, data, diagnostic.Bytes())
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func assertReferenceDeviceResults(t *testing.T, row map[string]json.RawMessage, output []byte) {
	t.Helper()

	var (
		result  icloud.GetAccountDevicesResult
		devices []icloud.AccountDevice
	)

	decode(t, output, &result)
	decode(t, row["devices"], &devices)

	if !reflect.DeepEqual(result.Devices, devices) {
		t.Fatal("reference-imported CLI device results changed")
	}
}

func assertSecondReferenceResume(t *testing.T, client icloud.Client, row map[string]json.RawMessage,
	logins []map[string]json.RawMessage, destination string, exchange replay.Exchange,
) {
	t.Helper()

	var output, diagnostic bytes.Buffer

	err := command.Run(t.Context(), client, []string{sessionFlag, destination, resumeCommand}, &output, &diagnostic)
	if err != nil || diagnostic.Len() != 0 {
		t.Fatalf("second saved-session resume: %v", err)
	}

	expected, err := json.Marshal(map[string]json.RawMessage{"auth_state": row["restoredAuthState"]})
	if err != nil {
		t.Fatal(err)
	}

	assertNativeSaved(t, map[string]json.RawMessage{"result": expected}, destination, output.Bytes())
	assertCompleteReferenceSession(t, row, logins, destination, "restoredAuthState", exchange)
}
