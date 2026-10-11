package replay_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const malformedDriveReceiptVariant = "malformed"

func TestDriveUploadStructuredReceipt(t *testing.T) {
	t.Parallel()

	for _, variant := range []string{"structured", "rejected", "missing-code", malformedDriveReceiptVariant} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			checkDriveStructuredReceipt(t, variant)
		})
	}
}

func checkDriveStructuredReceipt(t *testing.T, variant string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean("fixtures/sdk/drive-upload-" + variant + ".json"))
	if err != nil {
		t.Fatal(err)
	}

	var scenario driveUploadScenario

	err = json.Unmarshal(data, &scenario)
	if err != nil {
		t.Fatal(err)
	}

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithClock(func() time.Time {
		return time.Unix(scenario.Entropy.Seconds, 0)
	}))
	if err != nil {
		t.Fatal(err)
	}

	request, _ := sdkUploadRequest(t, scenario)
	result, err := client.UploadDriveFile(t.Context(), request)

	switch {
	case variant == malformedDriveReceiptVariant:
		var failure *icloud.ClientError

		if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse ||
			len(failure.PriorResponses()) != 2 {
			t.Fatal("malformed receipt must preserve completed stages and fail without retry", err)
		}
	case err != nil || result == nil:
		t.Fatal("accepted upload receipt was rejected", err)
	default:
		checkSDKMetadata(t, result.PreparationMetadata, scenario.Exchanges[0].Response)
		checkSDKMetadata(t, result.TransferMetadata, scenario.Exchanges[1].Response)
		checkSDKMetadata(t, result.RegistrationMetadata, scenario.Exchanges[2].Response)
		checkSDKValue(t, map[string]any{"statusDetails": result.StatusDetails, "results": result.Results,
			"documents": result.Documents, "additionalRegistrationMetadata": result.AdditionalRegistrationMetadata},
			scenario.Result)

		if result.Status != nil {
			t.Fatal("structured status was converted to an invented legacy status")
		}
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
