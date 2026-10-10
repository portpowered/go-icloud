package replay_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const photoControlUploadStages = 4
const photoControlCursorPrefix = "caller-prefix"

// These SDK edge controls extend Source's zero-cursor fixtures with caller-owned cursors.
func TestPhotoUploadHighLevelTransfersOnlyRemainingContent(t *testing.T) {
	t.Parallel()

	for _, fileOnly := range []bool{false, true} {
		for _, atEnd := range []bool{false, true} {
			name := "service-prefix"
			if fileOnly {
				name = "file-prefix"
			}
			if atEnd {
				name += "-end"
			}

			t.Run(name, func(t *testing.T) {
				t.Parallel()
				runPhotoUploadCursorControl(t, fileOnly, atEnd)
			})
		}
	}
}

func runPhotoUploadCursorControl(t *testing.T, fileOnly, atEnd bool) {
	t.Helper()
	filename := "photos-upload-service-ready.json"
	if fileOnly {
		filename = "photos-upload-pipeline-success.json"
	}

	path := filepath.Join(replayExpectedFixtures, replayExpectedSynthetic, "http", filename)
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)
	scenario.Exchanges = scenario.Exchanges[:photoControlUploadStages]
	_, content, _ := photoUploadContent(t, row)
	payload, err := io.ReadAll(content)
	if err != nil {
		t.Fatal(err)
	}

	reader := &photoUploadOwnedReader{
		Reader: bytes.NewReader(append([]byte(photoControlCursorPrefix), payload...)), closed: false,
	}
	position := int64(len(photoControlCursorPrefix))
	if atEnd {
		position += int64(len(payload))

		photoUploadEmptyRange(t, scenario.Exchanges)
	}

	_, err = reader.Seek(position, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	runner := newPhotoUploadReplay(t, row, scenario, transport)
	request := runner.uploadRequest(t)
	request.Content, request.Hydrate = reader, false
	if fileOnly {
		input := icloud.UploadPhotoFileRequest{Auth: request.Auth, Library: request.Library,
			Filename: request.Filename, Content: reader, ModificationTime: request.ModificationTime,
			LocalTimeZoneID: request.LocalTimeZoneID, TimeZoneOffset: request.TimeZoneOffset,
			ImportGroup: request.ImportGroup}
		result, callErr := runner.client.UploadPhotoFile(t.Context(), input)
		if callErr != nil || result == nil {
			t.Fatal("file upload failed remaining-content control", callErr)
		}
	} else {
		result, callErr := runner.client.UploadPhoto(t.Context(), request)
		if callErr != nil || result == nil {
			t.Fatal("service upload failed remaining-content control", callErr)
		}
	}

	position, err = reader.Seek(0, io.SeekCurrent)
	if err != nil || reader.closed || position != int64(len(photoControlCursorPrefix)+len(payload)) {
		t.Fatal("high-level upload lost reader cursor or ownership", position, err)
	}

	checkPhotoUploadControlConsumed(t, transport)
}

func photoUploadEmptyRange(t *testing.T, exchanges []replay.Exchange) {
	t.Helper()
	reserve := &exchanges[1].Request
	body := bytes.Replace(contractAuthBody(t, reserve.Body), []byte(": 3}"), []byte(": 0}"), 1)
	reserve.Body.Value = marshalFindMyRecovery(t, base64.StdEncoding.EncodeToString(body))
	transfer := &exchanges[2].Request
	transfer.Body.Value = marshalFindMyRecovery(t, "")
	transfer.Headers = []replay.Pair{{"accept", replayExpectedJSONMedia}, {chunkedHeader, "chunked"}}
	response := exchanges[2].Response
	body = bytes.Replace(contractAuthBody(t, response.Body), []byte(": 3,"), []byte(": 0,"), 1)
	response.Body.Value = marshalFindMyRecovery(t, base64.StdEncoding.EncodeToString(body))
	registration := &exchanges[3].Request
	body = bytes.Replace(contractAuthBody(t, registration.Body), []byte(": 3,"), []byte(": 0,"), 1)
	registration.Body.Value = marshalFindMyRecovery(t, base64.StdEncoding.EncodeToString(body))
}

func TestPhotoUploadHydrationBackoffCannotOverflow(t *testing.T) {
	t.Parallel()
	path := "fixtures/synthetic/http/photos-upload-service-delayed.json"
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)
	missing := scenario.Exchanges[photoControlUploadStages]
	ready := scenario.Exchanges[len(scenario.Exchanges)-1]
	scenario.Exchanges = append(scenario.Exchanges[:photoControlUploadStages+1], missing, ready)
	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	runner := newPhotoUploadReplay(t, row, scenario, transport)
	instant := runner.start
	waits := []time.Duration{}
	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(runner.entropy),
		icloud.WithClock(func() time.Time { return runner.start }),
		icloud.WithPhotoUploadClock(func() time.Time { return instant }),
		icloud.WithPhotoUploadWaiter(func(ctx context.Context, delay time.Duration) error {
			if delay < 0 {
				t.Fatal("hydration overflow produced a negative delay")
			}

			waits = append(waits, delay)
			instant = instant.Add(time.Nanosecond)

			return ctx.Err()
		}))
	if err != nil {
		t.Fatal(err)
	}

	request := runner.uploadRequest(t)
	maximum := time.Duration(math.MaxInt64)
	request.HydrationInterval, request.HydrationTimeout = &maximum, &maximum
	result, err := client.UploadPhoto(t.Context(), request)
	const maximumBackoff = 8 * time.Second
	if err != nil || result == nil || !result.Indexed || len(waits) != 2 || waits[0] != maximum ||
		waits[1] != maximumBackoff {
		t.Fatal("hydration delay was not safely saturated", waits, err)
	}

	checkPhotoUploadControlConsumed(t, transport)
}
