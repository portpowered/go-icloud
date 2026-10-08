package contracts_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/portpowered/go-icloud/internal/findmyapi"
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

	if pairs != 140 || jsonReplies != 127 {
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

func TestGeneratedFindMyCommandParsersPreserveReplyBytes(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, findMySchemaPath)

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/findmy-*.json")
	if err != nil {
		t.Fatal(err)
	}

	commands := 0

	for _, path := range paths {
		for _, exchange := range accountExchanges(t, path) {
			operation, err := bindFindMyOperation(document, exchange.Request)
			if err != nil {
				t.Fatal(err)
			}

			switch operation.OperationID {
			case "FindMyPlaySound", "FindMySendMessage", "FindMyLostDevice", "FindMyEraseDevice":
				body := findMyEntityBytes(t, exchange.Response.Body)
				response := new(http.Response)
				response.StatusCode = exchange.Response.Status

				response.Header = make(http.Header)

				for _, header := range exchange.Response.Headers {
					response.Header.Add(header[0], header[1])
				}

				response.Body = io.NopCloser(bytes.NewReader(body))

				parsed := parseFindMyCommandBody(t, operation.OperationID, response)
				if !bytes.Equal(parsed, body) {
					t.Fatal("generated command parser changed reply bytes")
				}

				commands++
			}
		}
	}

	if commands != 38 {
		t.Fatalf("command parser inventory changed: %d", commands)
	}
}

func parseFindMyCommandBody(t *testing.T, operation string, response *http.Response) []byte {
	t.Helper()

	switch operation {
	case "FindMyPlaySound":
		result, err := findmyapi.ParseFindMyPlaySoundResponse(response)
		if err != nil {
			t.Fatal(err)
		}

		return result.Body
	case "FindMySendMessage":
		result, err := findmyapi.ParseFindMySendMessageResponse(response)
		if err != nil {
			t.Fatal(err)
		}

		return result.Body
	case "FindMyLostDevice":
		result, err := findmyapi.ParseFindMyLostDeviceResponse(response)
		if err != nil {
			t.Fatal(err)
		}

		return result.Body
	case "FindMyEraseDevice":
		result, err := findmyapi.ParseFindMyEraseDeviceResponse(response)
		if err != nil {
			t.Fatal(err)
		}

		return result.Body
	default:
		t.Fatal("unknown generated command parser")

		return nil
	}
}
