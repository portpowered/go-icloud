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

var errSyntheticPeer = errors.New("synthetic peer failure")

type sdkRoundTrip func(*http.Request) (*http.Response, error)

func (roundTrip sdkRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func deviceRequest() icloud.GetAccountDevicesRequest {
	return icloud.GetAccountDevicesRequest{Auth: icloud.AuthContext{
		AccountID: "synthetic-account", ClientID: "synthetic-client",
		AccountServiceURL: "https://account.example.invalid", Headers: make([]icloud.Header, 0),
		ClientBuildNumber: nil, ClientMasteringNumber: nil, ChinaMainland: nil,
	}}
}

func sdkForResponse(t *testing.T, status int, body string) *icloud.SDK {
	t.Helper()

	client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
		response := new(http.Response)
		response.StatusCode = status
		response.Body = io.NopCloser(strings.NewReader(body))
		response.Header = make(http.Header)
		response.Header.Set("X-Synthetic", "synthetic-header")
		response.Header.Set("Content-Type", "application/json")

		return response, nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func TestDeviceProviderFailureKindsAndCopies(t *testing.T) {
	t.Parallel()

	for status, kind := range map[int]icloud.ErrorKind{
		http.StatusBadRequest: icloud.Provider, http.StatusUnauthorized: icloud.Unauthorized,
		http.StatusForbidden: icloud.Forbidden, http.StatusNotFound: icloud.NotFound,
		http.StatusTooManyRequests: icloud.RateLimited, http.StatusBadGateway: icloud.Unavailable,
		http.StatusServiceUnavailable: icloud.Unavailable, http.StatusGatewayTimeout: icloud.Unavailable,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			const body = `{"reason":"synthetic private provider content"}`

			client := sdkForResponse(t, status, body)
			response, err := client.GetAccountDevices(t.Context(), deviceRequest())

			failure := checkClientFailure(t, response, err, kind)
			if failure.StatusCode() != status || string(failure.ResponseBody()) != body {
				t.Fatal("provider failure lost its response")
			}

			copiedBody := failure.ResponseBody()
			copiedBody[0] = '!'
			headers := failure.ResponseHeaders()

			headers[1].Value = "changed"

			if string(failure.ResponseBody()) != body || failure.ResponseHeaders()[1].Value != "synthetic-header" {
				t.Fatal("error details leaked mutable storage")
			}
		})
	}
}

func TestDeviceInvalidProviderResponses(t *testing.T) {
	t.Parallel()

	for _, body := range []string{`{}`, `{"devices":null}`, `{"devices":[{"name":false}]}`, `not-json`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			client := sdkForResponse(t, http.StatusOK, body)
			response, err := client.GetAccountDevices(t.Context(), deviceRequest())

			failure := checkClientFailure(t, response, err, icloud.InvalidResponse)
			if string(failure.ResponseBody()) != body || failure.Unwrap() == nil {
				t.Fatal("invalid provider response lost body or cause")
			}
		})
	}
}

func TestDeviceTransportFailureCauses(t *testing.T) {
	t.Parallel()

	for kind, cause := range map[icloud.ErrorKind]error{
		icloud.Transport: errSyntheticPeer,
		icloud.Canceled:  context.Canceled, icloud.Timeout: context.DeadlineExceeded,
	} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()

			client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
				return nil, cause
			})))
			if err != nil {
				t.Fatal(err)
			}

			response, err := client.GetAccountDevices(t.Context(), deviceRequest())

			failure := checkClientFailure(t, response, err, kind)
			if !errors.Is(failure, cause) {
				t.Fatal("SDK lost original transport cause")
			}
		})
	}
}

func checkClientFailure(t *testing.T, response *icloud.GetAccountDevicesResult,
	err error, kind icloud.ErrorKind,
) *icloud.ClientError {
	t.Helper()

	var failure *icloud.ClientError

	if response != nil || !errors.As(err, &failure) || failure.Kind() != kind {
		t.Fatalf("SDK failure classification changed: %v", err)
	}

	if strings.Contains(err.Error(), "synthetic") {
		t.Fatal("SDK display error leaked private details")
	}

	return failure
}

func TestDeviceUnknownMetadataAndPayments(t *testing.T) {
	t.Parallel()

	const body = `{"devices":[{"name":"synthetic-device","future":{"big":9007199254740993,"nil":null}}],` +
		`"paymentMethods":[{"id":"synthetic-payment","isCarKey":true,"future":null}],` +
		`"futureAccount":[true,null,9007199254740993]}`

	client := sdkForResponse(t, http.StatusOK, body)
	response, err := client.GetAccountDevices(t.Context(), deviceRequest())

	if err != nil || response == nil {
		t.Fatalf("SDK metadata: %v", err)
	}

	if string(response.Devices[0].AdditionalProperties["future"]) != `{"big":9007199254740993,"nil":null}` ||
		string(response.AdditionalMetadata["futureAccount"]) != `[true,null,9007199254740993]` ||
		len(response.PaymentMethods) != 1 ||
		string(response.PaymentMethods[0].AdditionalProperties["future"]) != testJSONNull {
		t.Fatal("SDK metadata lost precision, null or payment details")
	}
}

func TestDeviceProviderErrorsInSuccessfulHTTPResponses(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{"devices":[],"errorMessage":"synthetic refusal"}`,
		`{"devices":[],"reason":"synthetic refusal"}`,
		`{"devices":[],"errorReason":"synthetic refusal"}`,
		`{"devices":[],"error":true}`, `{"devices":[],"error":1}`,
		`{"devices":[],"error":[1]}`, `{"devices":[],"error":{"failure":true}}`,
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			client := sdkForResponse(t, http.StatusOK, body)
			response, err := client.GetAccountDevices(t.Context(), deviceRequest())

			failure := checkClientFailure(t, response, err, icloud.Provider)
			if failure.StatusCode() != http.StatusOK || string(failure.ResponseBody()) != body {
				t.Fatal("success status masked a provider error envelope")
			}
		})
	}
}

func TestDeviceFalsyProviderFieldsRemainMetadata(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{`false`, testJSONNull, `""`, `0`, `[]`, `{}`} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			body := `{"devices":[],"error":` + raw + `}`
			client := sdkForResponse(t, http.StatusOK, body)

			response, err := client.GetAccountDevices(t.Context(), deviceRequest())
			if err != nil || response == nil || string(response.AdditionalMetadata["error"]) != raw {
				t.Fatalf("falsy metadata changed provider behavior: %v", err)
			}
		})
	}
}
