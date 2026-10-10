package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const writeFixtureDirectory = "../../../../tests/replay/fixtures/synthetic/http"

type writeFixture struct {
	Exchanges []replay.Exchange
	Inputs    []any
	Keywords  map[string]any
	Result    any
	Failure   json.RawMessage
	Initial   map[string]json.RawMessage
	Entropy   map[string]json.RawMessage
	File      map[string]json.RawMessage
}

type writeReplayCase struct {
	operation string
	fixture   string
}

func TestWriteCommandsPairedReferenceReplay(t *testing.T) {
	t.Parallel()

	for _, scenario := range writeReplayCases() {
		t.Run(scenario.fixture, func(t *testing.T) { t.Parallel(); runWriteReplay(t, scenario) })
	}
}

func writeReplayCases() []writeReplayCase {
	return []writeReplayCase{
		{testReminderCreateCommand, "reminders-create-basic"},
		{testReminderCreateCommand, "reminders-create-record-error"},
		{testReminderUpdateCommand, "reminders-update-basic"},
		{testReminderDeleteCommand, "reminders-delete-success"},
		{testReminderHashtagCreateCommand, "reminders-create-hashtag-success"},
		{testReminderHashtagUpdateCommand, "reminders-update-hashtag-success"},
		{testReminderHashtagDeleteCommand, "reminders-delete-hashtag-success"},
		{testReminderRecurrenceCreateCommand, "reminders-create-recurrence-rule-success"},
		{testReminderRecurrenceUpdateCommand, "reminders-update-recurrence-rule-success"},
		{testReminderRecurrenceDeleteCommand, "reminders-delete-recurrence-rule-success"},
		{testReminderAttachmentCreateCommand, "reminders-create-url-attachment-success"},
		{testReminderAttachmentUpdateCommand, "reminders-update-attachment-success"},
		{testReminderAttachmentDeleteCommand, "reminders-delete-attachment-success"},
		{testReminderLocationAddCommand, "reminders-add-location-trigger-success"},
		{testPhotoAlbumCreateCommand, "photos-create-album"},
		{testPhotoAlbumRenameCommand, "photos-rename-album"},
		{testPhotoAlbumDeleteCommand, "photos-delete-album"},
		{testPhotoAlbumAddCommand, "photos-add-to-album"},
		{testPhotoFavoriteCommand, "photos-favorite-true"},
		{testPhotoDeleteCommand, "photos-delete-asset"},
		{testPhotoFavoriteCommand, testPhotosFavoriteRecordError},
	}
}

func loadWriteFixture(t *testing.T, name string) writeFixture {
	t.Helper()

	data, err := readReplayFile(t, filepath.Join(writeFixtureDirectory, name+".json"))
	if err != nil {
		t.Fatal(err)
	}

	var row map[string]json.RawMessage

	decodeWriteFixture(t, data, &row)
	fixture := writeFixture{
		Exchanges: nil, Inputs: nil, Keywords: nil, Result: nil, Failure: row["error"],
		Initial: nil, Entropy: nil, File: nil,
	}
	decodeWriteFixture(t, row["exchanges"], &fixture.Exchanges)
	decodeWriteFixture(t, row["inputs"], &fixture.Inputs)
	decodeWriteFixture(t, row["initial_state"], &fixture.Initial)

	if len(row["entropy"]) != 0 {
		decodeWriteFixture(t, row["entropy"], &fixture.Entropy)
	}

	if len(row["file"]) != 0 {
		decodeWriteFixture(t, row["file"], &fixture.File)
	}

	if len(row[expectedReplayKeywordInputs]) != 0 {
		decodeWriteFixture(t, row[expectedReplayKeywordInputs], &fixture.Keywords)
	}

	if len(row["result"]) != 0 {
		decodeWriteFixture(t, row["result"], &fixture.Result)
	}

	return fixture
}

func decodeWriteFixture(t *testing.T, data []byte, target any) {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	err := decoder.Decode(target)
	if err != nil {
		t.Fatal(err)
	}
}

func fixtureWriteAuthentication(t *testing.T, fixture writeFixture) icloud.AuthContext {
	t.Helper()

	var params, headers map[string]string

	var origin string

	decodeWriteFixture(t, fixture.Initial["params"], &params)
	decodeWriteFixture(t, fixture.Initial["headers"], &headers)
	decodeWriteFixture(t, fixture.Initial["origin"], &origin)

	var auth icloud.AuthContext

	auth.AccountID, auth.ClientID = params["dsid"], params["clientId"]

	auth.RemindersServiceURL, auth.PhotosServiceURL = origin, origin
	for name, value := range headers {
		auth.Headers = append(auth.Headers, icloud.Header{Name: name, Value: value})
	}

	if value, exists := fixture.Initial["photos_upload_origin"]; exists {
		decodeWriteFixture(t, value, &auth.PhotosUploadServiceURL)
	}

	return auth
}

func fixtureWriteClient(t *testing.T, fixture writeFixture, transport *replay.HTTPTransport) *icloud.SDK {
	t.Helper()

	var identities []string

	var seconds int64

	if value, exists := fixture.Entropy["uuid4"]; exists {
		decodeWriteFixture(t, value, &identities)
	}

	if value, exists := fixture.Entropy["unix_seconds"]; exists {
		decodeWriteFixture(t, value, &seconds)
	}

	var random bytes.Buffer

	for _, identity := range identities {
		data, err := hex.DecodeString(strings.ReplaceAll(identity, "-", ""))
		if err != nil {
			t.Fatal(err)
		}

		random.Write(data)
	}

	if data, exists := fixture.Entropy["random_bytes"]; exists {
		var samples []string

		decodeWriteFixture(t, data, &samples)

		for _, sample := range samples {
			value, err := base64.StdEncoding.DecodeString(sample)
			if err != nil {
				t.Fatal(err)
			}

			random.Write(value)
		}
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(&random),
		icloud.WithClock(func() time.Time { return time.Unix(seconds, 0) }))
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func runWriteReplay(t *testing.T, scenario writeReplayCase) {
	t.Helper()
	fixture := loadWriteFixture(t, scenario.fixture)

	transport, err := replay.NewHTTPTransport(fixture.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client := fixtureWriteClient(t, fixture, transport)
	request := fixtureWriteInput(t, scenario.operation, fixture)
	// A foreign account in the request must never replace the selected stored session.
	request["auth"] = map[string]any{expectedClientID: "foreign", testAccountIDKey: "foreign", "headers": []any{}}
	directory := t.TempDir()
	session := filepath.Join(directory, expectedReplaySessionJSON)
	input := filepath.Join(directory, testRequestJSONFilename)
	result := filepath.Join(directory, expectedReplayResultJSON)

	writeFixtureValue(t, session, fixtureWriteAuthentication(t, fixture))
	writeFixtureValue(t, input, request)

	var output, diagnostic bytes.Buffer

	args := []string{sessionFlag, session, expectedRequestOption, input,
		expectedReplaySaveResult, result, scenario.operation}
	err = command.Run(t.Context(), client, args,
		&output, &diagnostic)
	checkWriteReplayResult(t, scenario, fixture, output.Bytes(), result, err)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func writeFixtureValue(t *testing.T, path string, value any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}

	writeProbeFile(t, path, data)
}

func writeProbeFile(t *testing.T, path string, data []byte) {
	t.Helper()

	err := os.WriteFile(path, data, 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

func checkWriteReplayResult(t *testing.T, scenario writeReplayCase, fixture writeFixture,
	output []byte, resultPath string, callErr error,
) {
	t.Helper()

	if len(fixture.Failure) != 0 || scenario.fixture == testPhotosFavoriteRecordError {
		var failure *icloud.ClientError

		if !errors.As(callErr, &failure) || len(output) != 0 {
			t.Fatalf("expected paired provider failure: %v", callErr)
		}

		if scenario.fixture == testPhotosFavoriteRecordError && failure.Kind() != icloud.Provider {
			t.Fatal("Photos per-record error exception lost provider classification")
		}

		checkFailedWriteReceipt(t, fixture, failure)

		_, statErr := os.Stat(resultPath)
		if !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatal("failed write saved a successful result")
		}

		return
	}

	if callErr != nil {
		t.Fatal(callErr)
	}

	expected := fixtureWriteExpected(t, scenario.operation, fixture)

	var actual any

	decodeWriteFixture(t, output, &actual)

	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Source console projection differs\nwant %#v\ngot %#v", expected, actual)
	}

	data, err := readReplayFile(t, resultPath)
	if err != nil {
		t.Fatal(err)
	}

	var saved map[string]json.RawMessage

	decodeWriteFixture(t, data, &saved)

	var responses []icloud.ResponseMetadata

	decodeWriteFixture(t, saved[expectedReplayResponses], &responses)
	checkWriteResponses(t, responses, fixture.Exchanges)
}

func checkFailedWriteReceipt(t *testing.T, fixture writeFixture, failure *icloud.ClientError) {
	t.Helper()

	last := fixture.Exchanges[len(fixture.Exchanges)-1]

	var encoded string

	decodeWriteFixture(t, last.Response.Body.Value, &encoded)

	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(failure.ResponseBody(), body) {
		t.Fatal("CLI failure lost paired provider body")
	}

	response := icloud.ResponseMetadata{
		StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL(),
	}
	checkWriteResponses(t, []icloud.ResponseMetadata{response}, []replay.Exchange{last})
	checkWriteResponses(t, failure.PriorResponses(), fixture.Exchanges[:len(fixture.Exchanges)-1])
}

func checkWriteResponses(t *testing.T, responses []icloud.ResponseMetadata, exchanges []replay.Exchange) {
	t.Helper()

	if len(responses) != len(exchanges) {
		t.Fatal("private result lost completed response evidence")
	}

	for index, response := range responses {
		paired := exchanges[index]
		if response.StatusCode != paired.Response.Status {
			t.Fatal("private response status changed")
		}

		headers := make([]icloud.Header, 0, len(paired.Response.Headers))
		for _, header := range paired.Response.Headers {
			headers = append(headers, icloud.Header{Name: header[0], Value: header[1]})
		}

		if !reflect.DeepEqual(response.Headers, headers) {
			t.Fatal("private result changed paired response headers")
		}

		address, err := url.Parse(paired.Request.Origin + paired.Request.Path)
		if err != nil {
			t.Fatal(err)
		}

		address.RawQuery = ""
		if response.CookieScopeURL != address.String() {
			t.Fatal("private result changed response cookie scope")
		}
	}
}
