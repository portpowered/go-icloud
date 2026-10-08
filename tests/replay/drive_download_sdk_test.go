package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type driveDownloadSnapshot struct {
	Status  int           `json:"status"`
	Headers []replay.Pair `json:"headers"`
	Body    string        `json:"body"`
}

func TestDriveSDKPortableDownloads(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/drive-*.json")
	if err != nil {
		t.Fatal(err)
	}

	selected := 0

	for _, path := range paths {
		data, readErr := os.ReadFile(filepath.Clean(path))
		if readErr != nil {
			t.Fatal(readErr)
		}

		var scenario driveReadScenario

		decodeErr := json.Unmarshal(data, &scenario)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}

		if scenario.Operation != "get_file" {
			continue
		}

		selected++

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runDriveSDKDownload(t, scenario)
		})
	}

	if selected != 7 {
		t.Fatal("Drive SDK download inventory changed")
	}
}

func runDriveSDKDownload(t *testing.T, scenario driveReadScenario) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	request := icloud.DownloadDriveFileRequest{Auth: sdkAccountAuth(scenario.Initial), DocumentID: "", Zone: nil}
	request.Auth.DriveDocumentServiceURL = scenario.Initial.DocumentOrigin

	err = json.Unmarshal(scenario.Inputs[0], &request.DocumentID)
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.DownloadDriveFile(t.Context(), request)
	if len(scenario.Error) != 0 {
		assertDriveDownloadFailure(t, scenario, result, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		assertDriveDownloadResult(t, scenario, result)

		checkSDKMetadata(t, result.TokenMetadata, scenario.Exchanges[0].Response)
		checkSDKMetadata(t, result.Metadata, scenario.Exchanges[1].Response)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func assertDriveDownloadResult(t *testing.T, scenario driveReadScenario, result *icloud.DownloadDriveFileResult) {
	t.Helper()

	headers := make([]replay.Pair, 0, len(result.Metadata.Headers))
	for _, header := range result.Metadata.Headers {
		headers = append(headers, replay.Pair{header.Name, header.Value})
	}

	value := driveDownloadSnapshot{Status: result.Metadata.StatusCode, Headers: headers,
		Body: base64.StdEncoding.EncodeToString(result.Content)}
	assertDriveSDKSuccess(t, scenario, value, result.TokenMetadata, nil)
}

func assertDriveDownloadFailure(t *testing.T, scenario driveReadScenario,
	result *icloud.DownloadDriveFileResult, err error,
) {
	t.Helper()

	var failure *icloud.ClientError

	if result != nil || !errors.As(err, &failure) {
		t.Fatalf("download leaked a partial result or untyped failure: %v", err)
	}

	exchange := scenario.Exchanges[len(scenario.Exchanges)-1].Response
	if exchange.Status == 200 {
		if failure.Kind() != icloud.InvalidResponse {
			t.Fatal("missing download URL lost classification")
		}
	} else {
		if failure.Kind() != icloud.Unavailable {
			t.Fatal("provider download failure lost classification")
		}

		accountProviderFailure(t, scenario.Error, failure.StatusCode(), failure.ResponseBody())
	}

	checkSDKMetadata(t, icloud.ResponseMetadata{StatusCode: failure.StatusCode(), Headers: failure.ResponseHeaders()},
		exchange)
}
