package replay_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type driveReadScenario struct {
	accountScenario

	Inputs []json.RawMessage `json:"inputs"`
}

func TestDriveSDKPortableReads(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(replayLiteralFixturesSyntheticHTTPDriveJSON)
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

		if scenario.Operation != "get_node_data" && scenario.Operation != replayLiteralGetAppData {
			continue
		}

		selected++

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runDriveSDKRead(t, scenario)
		})
	}

	if selected != 9 {
		t.Fatal("Drive SDK read inventory changed")
	}
}

func runDriveSDKRead(t *testing.T, scenario driveReadScenario) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	result, metadata, err := callDriveSDKRead(t, client, scenario)
	if len(scenario.Error) != 0 {
		assertDriveSDKFailure(t, scenario, err)
	} else {
		assertDriveSDKSuccess(t, scenario, result, metadata, err)
	}

	consumeErr := transport.AssertConsumed()
	if consumeErr != nil {
		t.Fatal(consumeErr)
	}
}

func assertDriveSDKFailure(t *testing.T, scenario driveReadScenario, err error) {
	t.Helper()

	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != sdkExpectedDeviceFailure(scenario.accountScenario) {
		t.Fatalf("Drive failure lost classification: %v", err)
	}

	accountProviderFailure(t, scenario.Error, failure.StatusCode(), failure.ResponseBody())
	checkSDKMetadata(t, icloud.ResponseMetadata{CookieScopeURL: failure.CookieScopeURL(), StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders()},
		scenario.Exchanges[0].Response)
}

func assertDriveSDKSuccess(t *testing.T, scenario driveReadScenario, result any,
	metadata icloud.ResponseMetadata, err error,
) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}

	encoded, encodeErr := json.Marshal(result)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}

	if !reflect.DeepEqual(accountJSON(t, encoded), accountJSON(t, scenario.Result)) {
		t.Fatalf("Drive public result changed: %s", encoded)
	}

	checkSDKMetadata(t, metadata, scenario.Exchanges[0].Response)
}

func callDriveSDKRead(t *testing.T, client icloud.Client,
	scenario driveReadScenario,
) (any, icloud.ResponseMetadata, error) {
	t.Helper()

	auth := sdkAccountAuth(scenario.Initial)
	auth.AccountServiceURL = ""
	auth.DriveServiceURL = scenario.Initial.Origin

	if scenario.Operation == replayLiteralGetAppData {
		result, err := client.ListDriveLibraries(t.Context(), icloud.ListDriveLibrariesRequest{Auth: auth})
		if err != nil {
			return nil, icloud.ResponseMetadata{}, fmt.Errorf(replayDriveReadErrorFormat, err)
		}

		return result.Libraries, result.Metadata, nil
	}

	request := icloud.GetDriveNodeRequest{Auth: auth, NodeID: "", ShareID: nil}

	decodeErr := json.Unmarshal(scenario.Inputs[0], &request.NodeID)
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}

	if len(scenario.Inputs) > 1 {
		decodeErr = json.Unmarshal(scenario.Inputs[1], &request.ShareID)
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
	}

	result, err := client.GetDriveNode(t.Context(), request)
	if err != nil {
		return nil, icloud.ResponseMetadata{}, fmt.Errorf(replayDriveReadErrorFormat, err)
	}

	return result.Node, result.Metadata, nil
}
