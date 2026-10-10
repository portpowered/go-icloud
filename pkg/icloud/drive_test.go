package icloud_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const (
	testDriveNodeID     = "synthetic-node"
	invalidProviderJSON = "not-json"
)

func driveAuth() icloud.AuthContext {
	auth := deviceRequest().Auth
	auth.AccountServiceURL = ""
	auth.DriveServiceURL = "https://drive.example.invalid"

	return auth
}

func TestDriveNodeRejectsInvalidProviderShapes(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`null`, `{}`, `[]`, `[null]`, `[false]`, `[{"name":false}]`, invalidProviderJSON} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			client := sdkForResponse(t, http.StatusOK, body)
			result, err := client.GetDriveNode(t.Context(), icloud.GetDriveNodeRequest{
				Auth: driveAuth(), NodeID: testDriveNodeID, ShareID: nil,
			})

			var failure *icloud.ClientError
			if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse ||
				string(failure.ResponseBody()) != body || failure.Unwrap() == nil {
				t.Fatal("Drive invalid response lost its body, classification or cause")
			}
		})
	}
}

func TestDriveLibrariesRejectsInvalidProviderShapes(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`null`, `{}`, `[]`, `{"items":null}`, testExpectedDriveFalseItems, `{"items":[{"name":false}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			client := sdkForResponse(t, http.StatusOK, body)
			result, err := client.ListDriveLibraries(t.Context(), icloud.ListDriveLibrariesRequest{Auth: driveAuth()})

			var failure *icloud.ClientError
			if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse ||
				string(failure.ResponseBody()) != body {
				t.Fatal("Drive invalid library envelope was accepted")
			}
		})
	}
}

func TestDriveNodeRetainsOptionalAndUnknownMetadata(t *testing.T) {
	t.Parallel()

	const body = `[{"size":"0","type":"FUTURE_KIND","shareID":{"owner":"synthetic-owner","future":null},` +
		`"future":18446744073709551615,"items":[{"future":[null,{"child":true}]}]}]`

	client := sdkForResponse(t, http.StatusOK, body)

	result, err := client.GetDriveNode(t.Context(), icloud.GetDriveNodeRequest{
		Auth: driveAuth(), NodeID: testDriveNodeID, ShareID: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	node := result.Node
	assertDriveScalarMetadata(t, node)

	if node.ShareID == nil || string(node.ShareID.AdditionalProperties["future"]) != testJSONNull {
		t.Fatal("Drive projection lost sharing metadata")
	}

	if node.Items == nil || len(*node.Items) != 1 ||
		string((*node.Items)[0].AdditionalProperties["future"]) != `[null,{"child":true}]` {
		t.Fatal("Drive recursive projection lost child metadata")
	}
}

func assertDriveScalarMetadata(t *testing.T, node icloud.DriveNode) {
	t.Helper()

	if node.Size == nil || string(*node.Size) != `"0"` || node.Type == nil || *node.Type != "FUTURE_KIND" ||
		string(node.AdditionalProperties["future"]) != testMaximumUint64 {
		t.Fatal("Drive projection lost scalar metadata")
	}
}

func TestDriveMissingChildrenRemainDistinctFromEmptyFolder(t *testing.T) {
	t.Parallel()

	client := sdkForResponse(t, http.StatusOK, `[{}]`)

	result, err := client.GetDriveNode(t.Context(), icloud.GetDriveNodeRequest{
		Auth: driveAuth(), NodeID: testDriveNodeID, ShareID: nil,
	})
	if err != nil || result.Node.Items != nil {
		t.Fatal("missing node children became an empty folder")
	}
}

func TestDriveRequestPreservesReferenceStringEncodingAndEmptyShareOmission(t *testing.T) {
	t.Parallel()

	const expected = `[{"drivewsid": "synthetic:<>&,\"\\:\u00e9\ud83d\ude00\u007f", "partialData": false}]`

	client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
		actual, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}

		if string(actual) != expected {
			t.Fatalf("reference JSON changed: %s", actual)
		}

		response := new(http.Response)
		response.StatusCode = http.StatusOK
		response.Header = make(http.Header)
		response.Header.Set(testExpectedContentType, testJSONMedia)
		response.Body = io.NopCloser(strings.NewReader(`[{}]`))

		return response, nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	share := new(icloud.DriveShareID)

	_, err = client.GetDriveNode(t.Context(), icloud.GetDriveNodeRequest{
		Auth: driveAuth(), NodeID: "synthetic:<>&,\"\\:é😀\x7f", ShareID: share,
	})
	if err != nil {
		t.Fatal(err)
	}
}
