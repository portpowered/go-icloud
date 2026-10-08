package icloud_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const testDriveDesiredName = "Synthetic"

func driveMutationInvocations() map[string]serviceInvocation {
	node := icloud.DriveNodeSelector{NodeID: testDriveNodeID, ETag: "synthetic-etag"}

	return map[string]serviceInvocation{
		"create": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.CreateDriveFolder(ctx, icloud.CreateDriveFolderRequest{
				Auth: auth, ParentID: testDriveNodeID, Name: testDriveDesiredName,
			})

			return result != nil, serviceCallError(err)
		},
		"rename": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.RenameDriveNode(ctx, icloud.RenameDriveNodeRequest{
				Auth: auth, Node: node, Name: testDriveDesiredName,
			})

			return result != nil, serviceCallError(err)
		},
		"move": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.MoveDriveNodes(ctx, icloud.MoveDriveNodesRequest{
				Auth: auth, DestinationID: testDriveNodeID, Nodes: []icloud.DriveNodeSelector{node},
			})

			return result != nil, serviceCallError(err)
		},
		"trash": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.TrashDriveNode(ctx, icloud.TrashDriveNodeRequest{Auth: auth, Node: node})

			return result != nil, serviceCallError(err)
		},
		"restore": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.RestoreDriveNode(ctx, icloud.RestoreDriveNodeRequest{Auth: auth, Node: node})

			return result != nil, serviceCallError(err)
		},
		"delete": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.DeleteDriveNode(ctx, icloud.DeleteDriveNodeRequest{Auth: auth, Node: node})

			return result != nil, serviceCallError(err)
		},
		"permanent": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.PermanentlyDeleteDriveNode(ctx, icloud.PermanentlyDeleteDriveNodeRequest{
				Auth: auth, Node: node,
			})

			return result != nil, serviceCallError(err)
		},
	}
}

func TestDriveMutationsOwnBodiesOnReadAndCloseFailure(t *testing.T) {
	t.Parallel()

	for name, invoke := range driveMutationInvocations() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body := new(failingAccountBody)

			client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
				response := new(http.Response)
				response.StatusCode = http.StatusOK
				response.Header = make(http.Header)
				response.Header.Set("Content-Type", "application/json")
				response.Body = body

				return response, nil
			})))
			if err != nil {
				t.Fatal(err)
			}

			received, err := invoke(t.Context(), client, driveAuth())

			var failure *icloud.ClientError

			if received || !body.closed || !errors.As(err, &failure) || failure.Kind() != icloud.Transport ||
				failure.StatusCode() != http.StatusOK || !errors.Is(err, errAccountRead) || !errors.Is(err, errAccountClose) {
				t.Fatal("mutation leaked its body, partial output or failure causes")
			}
		})
	}
}

func TestDriveMutationProviderShapes(t *testing.T) {
	t.Parallel()

	for name, bodies := range map[string][]string{
		"create": {testJSONNull, `[]`, invalidProviderJSON, `{"folders":false}`},
		"rename": {testJSONNull, `[]`, invalidProviderJSON, `{"items":false}`},
	} {
		invoke := driveMutationInvocations()[name]

		for _, body := range bodies {
			t.Run(name+body, func(t *testing.T) {
				t.Parallel()

				client := sdkForResponse(t, http.StatusOK, body)

				received, err := invoke(t.Context(), client, driveAuth())

				var failure *icloud.ClientError

				if received || !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse ||
					string(failure.ResponseBody()) != body || failure.Unwrap() == nil {
					t.Fatal("mutation accepted an invalid provider acknowledgement")
				}
			})
		}
	}
}

func TestDriveCreationOverridesContentTypeWithoutChangingCallerHeaders(t *testing.T) {
	t.Parallel()

	auth := driveAuth()
	auth.Headers = []icloud.Header{{Name: "Content-Type", Value: "application/json"},
		{Name: "Cookie", Value: "synthetic-private"}}

	client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Content-Type") != "plain/text" || request.Header.Get("Cookie") != "synthetic-private" {
			t.Fatal("creation did not apply its explicit reference header override")
		}

		response := new(http.Response)
		response.StatusCode = http.StatusOK
		response.Header = make(http.Header)
		response.Header.Set("Content-Type", "application/json")
		response.Body = io.NopCloser(strings.NewReader(`{"folders":[]}`))

		return response, nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.CreateDriveFolder(t.Context(), icloud.CreateDriveFolderRequest{
		Auth: auth, ParentID: testDriveNodeID, Name: testDriveDesiredName,
	})
	if err != nil || result.Folders == nil || len(*result.Folders) != 0 || auth.Headers[0].Value != "application/json" {
		t.Fatal("creation lost empty records or changed the caller's header state")
	}
}

func TestDriveMutationRetainsUnknownEnvelopeAndMissingItems(t *testing.T) {
	t.Parallel()

	const body = `{"future":18446744073709551615,"nullable":null}`

	client := sdkForResponse(t, http.StatusOK, body)

	result, err := client.RenameDriveNode(t.Context(), icloud.RenameDriveNodeRequest{
		Auth: driveAuth(), Node: icloud.DriveNodeSelector{NodeID: testDriveNodeID, ETag: ""}, Name: testDriveDesiredName,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Items != nil || string(result.AdditionalMetadata["future"]) != "18446744073709551615" ||
		string(result.AdditionalMetadata["nullable"]) != testJSONNull {
		t.Fatal("mutation lost missing-list presence or unknown JSON values")
	}
}
