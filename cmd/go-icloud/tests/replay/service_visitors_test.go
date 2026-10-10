package replay_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestPhotoVisitorCommandsPairedReferenceReplay(t *testing.T) {
	t.Parallel()

	for _, scenario := range []writeReplayCase{
		{"photo-assets-visit", "photos-assets-1"},
		{"photos-recently-added-visit", "photos-recently-added-one"},
	} {
		t.Run(scenario.fixture, func(t *testing.T) {
			t.Parallel()
			runPhotoVisitorReplay(t, scenario)
		})
	}
}

func runPhotoVisitorReplay(t *testing.T, scenario writeReplayCase) {
	t.Helper()
	fixture := loadWriteFixture(t, scenario.fixture)

	transport, err := replay.NewHTTPTransport(fixture.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client := fixtureWriteClient(t, fixture, transport)
	directory := t.TempDir()
	session := filepath.Join(directory, expectedReplaySessionJSON)
	request := filepath.Join(directory, testRequestJSONFilename)
	result := filepath.Join(directory, expectedReplayResultJSON)
	writeFixtureValue(t, session, fixtureWriteAuthentication(t, fixture))
	input := map[string]any{}
	if scenario.operation == "photo-assets-visit" {
		input["album"] = "Library"
	}

	writeFixtureValue(t, request, input)

	var output, diagnostic bytes.Buffer

	err = command.Run(t.Context(), client,
		[]string{sessionFlag, session, expectedRequestOption, request, expectedReplaySaveResult, result, scenario.operation},
		&output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	if diagnostic.Len() != 0 {
		t.Fatal("photo visitor emitted diagnostics")
	}

	checkPhotoVisitorSourceStream(t, fixture, &output)

	data, err := readReplayFile(t, result)
	if err != nil {
		t.Fatal(err)
	}

	var receipt icloud.ListPhotoAssetsResult

	decodeWriteFixture(t, data, &receipt)
	checkWriteResponses(t, receipt.Responses, fixture.Exchanges)
	if len(receipt.Photos) != len(sourceServiceList(t, fixture.Result)) {
		t.Fatal("photo visitor lost retained assets in private summary")
	}
}

func checkPhotoVisitorSourceStream(t *testing.T, fixture writeFixture, output io.Reader) {
	t.Helper()
	decoder := json.NewDecoder(output)
	decoder.UseNumber()

	photos := typedReadSourcePhotos(t, fixture.Result, false)
	for _, photo := range photos {
		var event any

		err := decoder.Decode(&event)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(event, map[string]any{"photo": photo}) {
			t.Fatalf("streamed photo differs from Source: %#v", event)
		}
	}

	var summary any

	err := decoder.Decode(&summary)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(summary, map[string]any{"photos": photos}) {
		t.Fatal("photo visitor summary differs", summary)
	}

	var extra any

	err = decoder.Decode(&extra)
	if !errors.Is(err, io.EOF) {
		t.Fatal("unexpected photo stream messages", err)
	}
}
