package contracts_test

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestFindMyModelsPreserveMetadataAndPresence(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		`{"content":[],"serverContext":null,"userInfo":null,"future":9007199254740993}`,
		`{"serverContext":{"theftLoss":null,"future":[null,false,9007199254740993]}}`,
		`{"content":[{"id":"synthetic","location":null,"msg":null,"remoteLock":null,` +
			`"deviceColor":null,"lockedTimestamp":null,"features":{"LOC":true,"future":null}}]}`,
		`{"content":[{"id":"synthetic","location":{"latitude":1.25,"longitude":2.5,` +
			`"timeStamp":1700000000000,"future":9007199254740993},"features":{}}]}`,
		`{"content":[{"id":"synthetic","batteryLevel":0.123456789,"location":{` +
			`"latitude":37.123456789,"longitude":-122.123456789,"altitude":123.123456789,` +
			`"horizontalAccuracy":1.123456789,"verticalAccuracy":2.123456789}}]}`,
	} {
		assertDriveRoundTrip(t, input, new(findmy.FindMyRefreshResponse))
	}

	assertDriveRoundTrip(t, `{"reason":null,"errorCode":null,"future":9007199254740993}`, new(findmy.FindMyError))
	assertDriveRoundTrip(t, `{"tokens":{"future":null}}`, new(findmy.FindMyEraseTokenResponse))
	assertDriveRoundTrip(t, `{"dsWebAuthToken":null}`, new(findmy.FindMyEraseTokenRequest))
}

func TestPortableFindMyRepliesRoundTripThroughCanonicalModels(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, findMySchemaPath)

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/findmy-*.json")
	if err != nil {
		t.Fatal(err)
	}

	pairs := 0
	jsonReplies := 0

	for _, path := range paths {
		for _, exchange := range accountExchanges(t, path) {
			operation, err := bindFindMyOperation(document, exchange.Request)
			if err != nil {
				t.Fatal(err)
			}

			body := findMyEntityBytes(t, exchange.Response.Body)
			if json.Valid(body) {
				assertDriveRoundTrip(t, string(body), findMyReplyTarget(operation.OperationID, exchange.Response.Status))

				jsonReplies++
			}

			pairs++
		}
	}

	if pairs != 70 || jsonReplies != 65 {
		t.Fatalf("Find My reply inventory changed: pairs=%d JSON=%d", pairs, jsonReplies)
	}
}

func findMyEntityBytes(t *testing.T, entity replay.Entity) []byte {
	t.Helper()

	if entity.Encoding != driveContractBinaryEncoding {
		return entity.Value
	}

	var encoded string

	err := json.Unmarshal(entity.Value, &encoded)
	if err != nil {
		t.Fatal(err)
	}

	value, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func findMyReplyTarget(operation string, status int) any {
	if status >= 400 {
		return new(findmy.FindMyError)
	}

	switch operation {
	case "FindMyInitialize", "FindMyRefreshDevices":
		return new(findmy.FindMyRefreshResponse)
	case "FindMyObtainEraseToken":
		return new(findmy.FindMyEraseTokenResponse)
	default:
		return new(findmy.FindMyAcknowledgement)
	}
}
