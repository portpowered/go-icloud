package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/photosync"
)

type sourceScenario struct {
	Name    string              `json:"name"`
	Options json.RawMessage     `json:"options"`
	Assets  []photosync.Asset   `json:"assets"`
	Result  json.RawMessage     `json:"result"`
	Files   map[string]string   `json:"files"`
	Calls   map[string][]string `json:"calls"`
}

type sourceRecording struct {
	Provenance json.RawMessage  `json:"provenance"`
	Scenarios  []sourceScenario `json:"scenarios"`
}

func TestPinnedSourceFilesystemReplay(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("fixtures/synthetic/filesystem/photos-sync.json")
	if err != nil {
		t.Fatal(err)
	}

	var recording sourceRecording

	err = json.Unmarshal(data, &recording)
	if err != nil {
		t.Fatal(err)
	}

	for _, scenario := range recording.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) { t.Parallel(); replaySync(t, scenario) })
	}
}

func replaySync(t *testing.T, scenario sourceScenario) {
	t.Helper()

	provider := newSource(scenario.Assets...)

	input := request(filepath.Join(t.TempDir(), "output"))

	err := json.Unmarshal(scenario.Options, &input.Options)
	if err != nil {
		t.Fatal(err)
	}

	result, err := newEngine(t, provider).Run(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}

	actual, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}

	assertSourceResult(t, actual, scenario.Result)

	for path, encoded := range scenario.Files {
		data, readErr := os.ReadFile(filepath.Join(input.Options.Directory, filepath.FromSlash(path)))
		if readErr != nil {
			t.Fatal(readErr)
		}

		if base64.StdEncoding.EncodeToString(data) != encoded {
			t.Fatalf("file %s differs from Source", path)
		}
	}

	assertSourceCalls(t, provider, scenario.Calls)
}

func assertSourceResult(t *testing.T, actual, expected []byte) {
	t.Helper()

	var got, want map[string]json.RawMessage

	err := json.Unmarshal(actual, &got)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(expected, &want)
	if err != nil {
		t.Fatal(err)
	}

	for key, value := range want {
		var gotValue, wantValue any

		err := json.Unmarshal(got[key], &gotValue)
		if err != nil {
			t.Fatal(err)
		}

		err = json.Unmarshal(value, &wantValue)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(gotValue, wantValue) {
			t.Errorf("Source %s: got %s want %s", key, got[key], value)
		}
	}
}

func assertSourceCalls(t *testing.T, provider *source, expected map[string][]string) {
	t.Helper()

	actual := map[string][]string{}
	for assetID := range expected {
		actual[assetID] = []string{}
	}

	for _, download := range provider.downloads {
		parts := strings.SplitN(download, ":", 2)
		actual[parts[0]] = append(actual[parts[0]], "download:"+parts[1])
	}

	for _, assetID := range provider.deletions {
		actual[assetID] = append(actual[assetID], "delete")
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Source call transcript: got %+v want %+v", actual, expected)
	}
}
