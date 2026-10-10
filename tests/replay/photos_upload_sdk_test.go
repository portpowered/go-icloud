package replay_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	photoUploadReserveOperation  = "upload_reserve"
	photoUploadBytesOperation    = "upload_bytes"
	photoUploadRegisterOperation = "upload_register"
	photoUploadStatusOperation   = "upload_status"
	photoUploadPipelineOperation = "upload_pipeline"
	photoUploadServiceOperation  = "service_upload"
	photoUploadNull              = "null"
)

type photoUploadReplayResult struct {
	value     any
	responses []icloud.ResponseMetadata
}

type photoUploadReplayClock struct {
	instant  time.Time
	trace    []photoUploadReplayWait
	expected []photoUploadReplayWait
}

type photoUploadReplayWait struct {
	Kind  string  `json:"kind"`
	Value float64 `json:"value"`
}

type photoUploadReplay struct {
	row      map[string]json.RawMessage
	scenario accountScenario
	client   *icloud.SDK
	auth     icloud.AuthContext
	entropy  *bytes.Buffer
	clock    *photoUploadReplayClock
	start    time.Time
}

func TestPhotoUploadSDKPortableScenarios(t *testing.T) {
	t.Parallel()
	paths, err := filepath.Glob("fixtures/synthetic/http/photos-upload-*.json")
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, path := range paths {
		if strings.Contains(filepath.Base(path), "-shared-") {
			continue
		}

		count++

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runPhotoUploadSDK(t, path)
		})
	}

	if count != 46 {
		t.Fatalf("Photos upload scenario inventory changed: %d", count)
	}
}

func runPhotoUploadSDK(t *testing.T, path string) {
	t.Helper()
	row := authReplayObject(t, path)
	scenario := readAccountScenario(t, path)
	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	runner := newPhotoUploadReplay(t, row, scenario, transport)
	authBefore := marshalFindMyRecovery(t, runner.auth)
	result, callErr := runner.call(t)
	checkSDKValue(t, runner.auth, authBefore)
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}

	checkPhotoUploadOutcome(t, runner, result, callErr)
	runner.checkEntropyAndWaits(t)
}

func newPhotoUploadReplay(t *testing.T, row map[string]json.RawMessage,
	scenario accountScenario, transport *replay.HTTPTransport,
) *photoUploadReplay {
	t.Helper()
	var entropy struct {
		UUIDs []string `json:"uuid4"`
		//nolint:tagliatelle // LIB-05: pinned reference fixture spelling.
		Seconds int64 `json:"unix_seconds"`
		//nolint:tagliatelle // LIB-05: pinned reference fixture spelling.
		WaitTrace []photoUploadReplayWait `json:"photos_wait_trace"`
	}

	if raw := row["entropy"]; len(raw) != 0 {
		authReplayDecode(t, raw, &entropy)
	}

	random := new(bytes.Buffer)
	for _, identity := range entropy.UUIDs {
		data, err := hex.DecodeString(strings.ReplaceAll(identity, "-", ""))
		if err != nil {
			t.Fatal(err)
		}

		random.Write(data)
	}

	start := time.Unix(entropy.Seconds, 0)
	clock := &photoUploadReplayClock{instant: start, trace: []photoUploadReplayWait{}, expected: entropy.WaitTrace}
	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(random),
		icloud.WithClock(func() time.Time { return start }), icloud.WithPhotoUploadClock(func() time.Time {
			if len(clock.trace) < len(clock.expected) && clock.expected[len(clock.trace)].Kind == "monotonic" {
				clock.instant = start.Add(time.Duration(clock.expected[len(clock.trace)].Value * float64(time.Second)))
			}
			elapsed := clock.instant.Sub(start).Seconds()
			clock.trace = append(clock.trace, photoUploadReplayWait{Kind: "monotonic", Value: elapsed})

			return clock.instant
		}), icloud.WithPhotoUploadWaiter(func(ctx context.Context, delay time.Duration) error {
			err := ctx.Err()
			if err != nil {
				return fmt.Errorf("upload replay waiter: %w", err)
			}

			clock.trace = append(clock.trace, photoUploadReplayWait{Kind: "sleep", Value: delay.Seconds()})
			clock.instant = clock.instant.Add(delay)

			return nil
		}))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	var initial map[string]json.RawMessage

	authReplayDecode(t, row["initial_state"], &initial)
	authReplayDecode(t, initial["photos_upload_origin"], &auth.PhotosUploadServiceURL)

	return &photoUploadReplay{row: row, scenario: scenario, client: client, auth: auth,
		entropy: random, clock: clock, start: start}
}

//nolint:wrapcheck // LIB-05: retain SDK failures unchanged for complete evidence assertions.
func (runner *photoUploadReplay) call(t *testing.T) (*photoUploadReplayResult, error) {
	t.Helper()

	switch runner.scenario.Operation {
	case photoUploadReserveOperation:
		var keywords struct {
			Assets map[string]int64 `json:"assets"`
		}

		authReplayDecode(t, runner.row["keyword_inputs"], &keywords)
		result, err := runner.client.ReservePhotoUploads(t.Context(), icloud.ReservePhotoUploadsRequest{
			Auth: runner.auth, Library: nil, Assets: maps.Clone(keywords.Assets)})
		if result == nil {
			return nil, err
		}

		return &photoUploadReplayResult{value: result.UploadURLs, responses: result.Responses}, err
	case photoUploadBytesOperation:
		var inputs []string

		authReplayDecode(t, runner.row["inputs"], &inputs)
		_, content, _ := photoUploadContent(t, runner.row)
		result, err := runner.client.SendPhotoUploadBytes(t.Context(), icloud.SendPhotoUploadBytesRequest{
			Auth: runner.auth, Library: nil, URL: inputs[0], Content: content})
		if result == nil {
			return nil, err
		}

		return &photoUploadReplayResult{value: result.Receipt, responses: result.Responses}, err
	case photoUploadRegisterOperation:
		return runner.register(t)
	case photoUploadStatusOperation:
		var inputs [][]string

		authReplayDecode(t, runner.row["inputs"], &inputs)
		result, err := runner.client.GetPhotoUploadStatus(t.Context(), icloud.GetPhotoUploadStatusRequest{
			Auth: runner.auth, Library: nil, JobIDs: inputs[0]})
		if result == nil {
			return nil, err
		}

		return &photoUploadReplayResult{value: result.Jobs, responses: result.Responses}, err
	default:
		return runner.upload(t)
	}
}

//nolint:wrapcheck // LIB-05: preserve the SDK failure without changing its identity.
func (runner *photoUploadReplay) upload(t *testing.T) (*photoUploadReplayResult, error) {
	t.Helper()
	if runner.scenario.Operation == photoUploadPipelineOperation {
		request := runner.uploadRequest(t)
		result, err := runner.client.UploadPhotoFile(t.Context(), icloud.UploadPhotoFileRequest{
			Auth: request.Auth, Content: request.Content, Filename: request.Filename,
			ImportGroup: request.ImportGroup, Library: request.Library, LocalTimeZoneID: request.LocalTimeZoneID,
			ModificationTime: request.ModificationTime, TimeZoneOffset: request.TimeZoneOffset})
		if result == nil {
			return nil, err
		}
		return &photoUploadReplayResult{value: result, responses: result.Responses}, err
	}

	request := runner.uploadRequest(t)
	result, err := runner.client.UploadPhoto(t.Context(), request)
	if result == nil {
		return nil, err
	}

	return &photoUploadReplayResult{value: result, responses: result.Responses}, err
}

//nolint:wrapcheck // LIB-05: preserve the exact SDK error for failure-stage assertions.
func (runner *photoUploadReplay) register(t *testing.T) (*photoUploadReplayResult, error) {
	t.Helper()
	var keywords struct {
		Files []struct {
			Filename string                    `json:"fileName"`
			Modified int64                     `json:"lastModDate"`
			Offset   int                       `json:"timeZoneOffset"`
			Receipt  icloud.PhotoUploadReceipt `json:"singleFileUploadRequest"`
		} `json:"files"`
		//nolint:tagliatelle // LIB-05: pinned reference fixture spelling.
		Group string `json:"import_group"`
		//nolint:tagliatelle // LIB-05: pinned reference fixture spelling.
		Zone string `json:"local_time_zone_id"`
	}

	authReplayDecode(t, runner.row["keyword_inputs"], &keywords)

	files := make([]icloud.PhotoUploadFile, 0, len(keywords.Files))

	for _, file := range keywords.Files {
		files = append(files, icloud.PhotoUploadFile{Filename: file.Filename, ModificationTime: time.UnixMilli(file.Modified),
			TimeZoneOffset: file.Offset, Receipt: file.Receipt})
	}

	request := icloud.RegisterPhotoUploadsRequest{Auth: runner.auth, Library: nil, Files: files,
		ImportGroup: keywords.Group, LocalTimeZoneID: keywords.Zone}
	before := marshalFindMyRecovery(t, request)
	result, err := runner.client.RegisterPhotoUploads(t.Context(), request)
	checkSDKValue(t, request, before)
	if result == nil {
		return nil, err
	}

	return &photoUploadReplayResult{value: result.Registrations, responses: result.Responses}, err
}

func photoUploadContent(t *testing.T, row map[string]json.RawMessage) (string, *bytes.Reader, time.Time) {
	t.Helper()
	var file struct {
		Name string `json:"name"`
		Body string `json:"body"`
		//nolint:tagliatelle // LIB-05: pinned reference fixture spelling.
		Seconds int64 `json:"modified_seconds"`
	}

	authReplayDecode(t, row["file"], &file)
	data, err := base64.StdEncoding.DecodeString(file.Body)
	if err != nil {
		t.Fatal(err)
	}

	return file.Name, bytes.NewReader(data), time.Unix(file.Seconds, 0)
}

func (runner *photoUploadReplay) uploadRequest(t *testing.T) icloud.UploadPhotoRequest {
	t.Helper()
	request := new(icloud.UploadPhotoRequest)
	request.Auth = runner.auth
	request.Filename, request.Content, request.ModificationTime = photoUploadContent(t, runner.row)
	request.Hydrate = runner.scenario.Operation == photoUploadServiceOperation
	request.LocalTimeZoneID = "UTC"
	var entropy, initial, keywords map[string]json.RawMessage

	if raw := runner.row["entropy"]; len(raw) != 0 {
		authReplayDecode(t, raw, &entropy)
	}

	if raw := entropy["photos_local_timezone"]; len(raw) != 0 {
		var parts []json.RawMessage

		authReplayDecode(t, raw, &parts)
		authReplayDecode(t, parts[0], &request.LocalTimeZoneID)
		authReplayDecode(t, parts[1], &request.TimeZoneOffset)
	}

	authReplayDecode(t, runner.row["initial_state"], &initial)
	request.HydrationTimeout = photoUploadDuration(t, initial["upload_hydration_timeout"])
	request.HydrationInterval = photoUploadDuration(t, initial["upload_hydration_interval"])
	authReplayDecode(t, runner.row["keyword_inputs"], &keywords)
	if raw := keywords["album"]; len(raw) != 0 {
		var name string

		authReplayDecode(t, raw, &name)
		request.AlbumID = &name
	}

	return *request
}

func photoUploadDuration(t *testing.T, raw json.RawMessage) *time.Duration {
	t.Helper()
	if len(raw) == 0 {
		return nil
	}

	var seconds float64

	authReplayDecode(t, raw, &seconds)
	value := time.Duration(seconds * float64(time.Second))

	return &value
}

func (runner *photoUploadReplay) checkEntropyAndWaits(t *testing.T) {
	t.Helper()
	if runner.entropy.Len() != 0 {
		t.Fatal("upload did not consume declared identities")
	}

	var entropy map[string]json.RawMessage

	if raw := runner.row["entropy"]; len(raw) != 0 {
		authReplayDecode(t, raw, &entropy)
	}

	if expected := entropy["photos_wait_trace"]; len(expected) != 0 {
		checkSDKValue(t, runner.clock.trace, expected)
	} else if len(runner.clock.trace) != 0 {
		t.Fatal("upload performed an undeclared clock read or wait")
	}
}

func checkPhotoUploadOutcome(t *testing.T, runner *photoUploadReplay,
	result *photoUploadReplayResult, callErr error,
) {
	t.Helper()
	if len(runner.scenario.Error) != 0 {
		if result != nil {
			t.Fatal("upload returned an acknowledgement on failure")
		}

		checkPhotoUploadFailure(t, runner.scenario, callErr)

		return
	}

	if result == nil || callErr != nil {
		t.Fatal("upload did not produce its Source result", callErr, errors.Unwrap(callErr))
	}

	switch runner.scenario.Operation {
	case photoUploadRegisterOperation:
		checkSDKValue(t, result.value, photoUploadRegistrations(t, runner.scenario.Result))
	case photoUploadStatusOperation:
		checkSDKValue(t, result.value, photoUploadStatuses(t, runner.scenario.Result))
	case photoUploadPipelineOperation, photoUploadServiceOperation:
		checkPhotoUploadPipeline(t, runner.scenario, result.value)
	default:
		checkSDKValue(t, result.value, runner.scenario.Result)
	}

	checkPhotoUploadResponses(t, result.responses, runner.scenario.Exchanges)
}

func photoUploadRegistration(t *testing.T, raw json.RawMessage) json.RawMessage {
	t.Helper()
	var source map[string]json.RawMessage

	authReplayDecode(t, raw, &source)
	for old, field := range map[string]string{"uploadJobId": "jobID", "cplMaster": "masterID", "cplAsset": "photoID"} {
		source[field] = source[old]
		if len(source[field]) == 0 {
			source[field] = json.RawMessage(photoUploadNull)
		}
		delete(source, old)
	}

	response := source["response"]
	delete(source, "response")
	duplicate := false
	if len(response) != 0 && string(response) != photoUploadNull {
		var status map[string]json.RawMessage

		authReplayDecode(t, response, &status)
		status["retryable"] = status["isRetryable"]

		for _, name := range []string{"retryable", "status", "errorMessage"} {
			if len(status[name]) == 0 {
				status[name] = json.RawMessage(photoUploadNull)
			}
		}
		delete(status, "isRetryable")
		source["status"] = marshalFindMyRecovery(t, status)
		duplicate = string(status["status"]) == "409"
	} else {
		source["status"] = json.RawMessage(photoUploadNull)
	}

	source["duplicate"] = marshalFindMyRecovery(t, duplicate)

	return marshalFindMyRecovery(t, source)
}

func photoUploadRegistrations(t *testing.T, raw json.RawMessage) json.RawMessage {
	t.Helper()
	var source []json.RawMessage

	authReplayDecode(t, raw, &source)
	for index, item := range source {
		source[index] = photoUploadRegistration(t, item)
	}

	return marshalFindMyRecovery(t, source)
}

func photoUploadStatuses(t *testing.T, raw json.RawMessage) json.RawMessage {
	t.Helper()
	var source map[string]struct {
		Value map[string]json.RawMessage `json:"value"`
		//nolint:tagliatelle // LIB-05: pinned reference fixture spelling.
		Unknown bool `json:"is_unknown"`
	}

	authReplayDecode(t, raw, &source)

	result := make(map[string]map[string]json.RawMessage, len(source))
	for name, item := range source {
		item.Value["unknown"] = marshalFindMyRecovery(t, item.Unknown)
		result[name] = item.Value
	}

	return marshalFindMyRecovery(t, result)
}

func checkPhotoUploadPipeline(t *testing.T, scenario accountScenario, actual any) {
	t.Helper()
	if scenario.Operation == photoUploadPipelineOperation {
		result, ok := actual.(*icloud.UploadPhotoFileResult)
		if !ok {
			t.Fatal("pipeline result does not own typed registration")
		}
		var expected struct {
			Value json.RawMessage `json:"value"`
			//nolint:tagliatelle // LIB-05: pinned reference fixture spelling.
			Duplicate bool `json:"is_duplicate"`
		}

		authReplayDecode(t, scenario.Result, &expected)
		checkSDKValue(t, result.Registration, photoUploadRegistration(t, expected.Value))
		if result.Registration.Duplicate != expected.Duplicate {
			t.Fatal("pipeline changed Source duplicate or indexing state")
		}

		return
	}
	result, ok := actual.(*icloud.UploadPhotoResult)
	if !ok {
		t.Fatal("service result does not own typed registration and photo")
	}

	checkPhotoUploadServiceRegistration(t, scenario, result.Registration)

	if string(scenario.Result) == photoUploadNull {
		if result.Indexed || !result.Photo.IsNull() {
			t.Fatal("unindexed upload returned a hydrated photo")
		}

		return
	}

	if !result.Indexed || result.Photo.IsNull() {
		t.Fatal("indexed upload lost its hydrated photo")
	}

	checkPhotoAssetsProjection(t, []icloud.Photo{result.Photo.GetOrEmpty()},
		marshalFindMyRecovery(t, []json.RawMessage{scenario.Result}))
}

func checkPhotoUploadFailure(t *testing.T, scenario accountScenario, callErr error) {
	t.Helper()
	var failure *icloud.ClientError

	if !errors.As(callErr, &failure) {
		t.Fatal("upload did not return a typed failure", callErr)
	}

	last := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	kind := icloud.InvalidResponse

	switch {
	case last.Status == http.StatusServiceUnavailable:
		kind = icloud.Unavailable
	case last.Status == http.StatusTooManyRequests:
		kind = icloud.RateLimited
	case strings.Contains(string(scenario.Error), "rejected"):
		kind = icloud.Provider
	case strings.Contains(string(scenario.Error), "No album matched"):
		kind = icloud.NotFound
	}

	if failure.Kind() != kind || !bytes.Equal(failure.ResponseBody(), contractAuthBody(t, last.Body)) ||
		len(failure.PriorResponses()) != len(scenario.Exchanges)-1 {
		t.Fatal("upload failure lost its stage, body or prior responses", failure.Kind(), kind, callErr)
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders(), CookieScopeURL: failure.CookieScopeURL()}, last)
	lastRequest := scenario.Exchanges[len(scenario.Exchanges)-1].Request
	if failure.CookieScopeURL() != lastRequest.Origin+lastRequest.Path {
		t.Fatal("upload failure lost exact cookie scope")
	}

	checkPhotoUploadResponses(t, failure.PriorResponses(), scenario.Exchanges[:len(scenario.Exchanges)-1])
}

func checkPhotoUploadResponses(t *testing.T, actual []icloud.ResponseMetadata, exchanges []replay.Exchange) {
	t.Helper()
	checkReminderSyncResponses(t, actual, exchanges)
	for index, response := range actual {
		request := exchanges[index].Request
		if response.CookieScopeURL != request.Origin+request.Path {
			t.Fatal("upload response lost exact cookie scope", index)
		}
	}
}

func checkPhotoUploadServiceRegistration(t *testing.T,
	scenario accountScenario, actual icloud.PhotoUploadRegistration,
) {
	t.Helper()
	for _, exchange := range scenario.Exchanges {
		if exchange.Request.Path != protocol.PhotosUploadPhotosPutAssetPath {
			continue
		}

		var rows []json.RawMessage

		authReplayDecode(t, contractAuthBody(t, exchange.Response.Body), &rows)
		if len(rows) != 1 {
			t.Fatal("successful service upload did not own one registration")
		}

		checkSDKValue(t, actual, photoUploadRegistration(t, rows[0]))

		return
	}

	t.Fatal("successful service upload has no registration response")
}
