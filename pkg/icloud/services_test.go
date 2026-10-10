package icloud_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const testJSONNull = "null"

var (
	errAccountRead  = errors.New("synthetic response read failed")
	errAccountClose = errors.New("synthetic response close failed")
)

type serviceInvocation func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (received bool, err error)

func accountServiceInvocations() map[string]serviceInvocation {
	return map[string]serviceInvocation{
		"family": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.GetAccountFamily(ctx, icloud.GetAccountFamilyRequest{Auth: auth})

			return result != nil, serviceCallError(err)
		},
		"photo": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.GetAccountMemberPhoto(ctx, icloud.GetAccountMemberPhotoRequest{
				Auth: auth, MemberID: "synthetic-member",
			})

			return result != nil, serviceCallError(err)
		},
		"storage": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.GetAccountStorage(ctx, icloud.GetAccountStorageRequest{Auth: auth})

			return result != nil, serviceCallError(err)
		},
		"plan": func(ctx context.Context, client *icloud.SDK, auth icloud.AuthContext) (bool, error) {
			result, err := client.GetAccountPlanSummary(ctx, icloud.GetAccountPlanSummaryRequest{Auth: auth})

			return result != nil, serviceCallError(err)
		},
	}
}

type failingAccountBody struct{ closed bool }

func (*failingAccountBody) Read(_ []byte) (int, error) { return 0, errAccountRead }
func (body *failingAccountBody) Close() error {
	body.closed = true

	return errAccountClose
}

func TestAccountServicesOwnBodiesOnReadAndCloseFailure(t *testing.T) {
	t.Parallel()

	for name, invoke := range accountServiceInvocations() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body := new(failingAccountBody)

			client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
				response := new(http.Response)
				response.StatusCode = http.StatusOK
				response.Header = make(http.Header)
				response.Header.Set("Content-Type", testJSONMedia)
				response.Body = body

				return response, nil
			})))
			if err != nil {
				t.Fatal(err)
			}

			received, err := invoke(t.Context(), client, deviceRequest().Auth)

			var failure *icloud.ClientError

			if received || !body.closed || !errors.As(err, &failure) || failure.Kind() != icloud.Transport ||
				failure.StatusCode() != http.StatusOK || !errors.Is(err, errAccountRead) || !errors.Is(err, errAccountClose) {
				t.Fatal("account service did not close its body, suppress output or retain both failure causes")
			}
		})
	}
}

func TestAccountServicesRejectInvalidProviderShapes(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"family": {testJSONNull, `[]`, `{"familyMembers":null}`, `{"familyMembers":[{"fullName":false}]}`},
		"storage": {testJSONNull, `{}`, `{"storageUsageInfo":{}}`,
			`{"storageUsageInfo":{"usedStorageInBytes":null,"totalStorageInBytes":100}}`,
			`{"storageUsageInfo":{"usedStorageInBytes":-1,"totalStorageInBytes":100}}`,
			`{"storageUsageInfo":{"usedStorageInBytes":0,"totalStorageInBytes":100},"storageUsageByMedia":[{}]}`},
		"plan": {invalidProviderJSON},
	}

	for name, bodies := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			for _, body := range bodies {
				client := sdkForResponse(t, http.StatusOK, body)
				received, err := accountServiceInvocations()[name](t.Context(), client, deviceRequest().Auth)

				var failure *icloud.ClientError

				if received || !errors.As(err, &failure) || failure.Kind() != icloud.InvalidResponse ||
					string(failure.ResponseBody()) != body || failure.StatusCode() != http.StatusOK {
					t.Fatalf("invalid %s provider body: %v", name, err)
				}
			}
		})
	}
}

func TestMemberPhotoProviderErrorEnvelope(t *testing.T) {
	t.Parallel()

	client := sdkForResponse(t, http.StatusOK, `{"reason":"synthetic refusal"}`)
	received, err := accountServiceInvocations()["photo"](t.Context(), client, deviceRequest().Auth)

	var failure *icloud.ClientError

	if received || !errors.As(err, &failure) || failure.Kind() != icloud.Provider {
		t.Fatal("photo returned an error envelope as image bytes")
	}
}

func TestOpaquePlanJSONKeepsItsExactBytes(t *testing.T) {
	t.Parallel()

	const body = "  [null,9007199254740993,true] \n"

	client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
		response := new(http.Response)
		response.StatusCode = http.StatusOK
		response.Header = make(http.Header)
		response.Body = io.NopCloser(strings.NewReader(body))

		return response, nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.GetAccountPlanSummary(t.Context(), icloud.GetAccountPlanSummaryRequest{
		Auth: deviceRequest().Auth,
	})
	if err != nil || string(result.Summary) != body {
		t.Fatal("opaque subscription JSON lost whitespace, null or integer precision")
	}
}

func serviceCallError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("account service invocation: %w", err)
}
