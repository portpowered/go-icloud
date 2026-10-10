package replay_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestAccountDevicesSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/account-devices-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 11 {
		t.Fatal("SDK device scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			accountDevicesSDK(t, readAccountScenario(t, path))
		})
	}
}

func accountDevicesSDK(t *testing.T, scenario accountScenario) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.GetAccountDevices(t.Context(), icloud.GetAccountDevicesRequest{
		Auth: sdkAccountAuth(scenario.Initial),
	})
	if len(scenario.Error) != 0 {
		assertDevicesSDKFailure(t, scenario, response, err)
	} else {
		assertDevicesSDKSuccess(t, scenario, response, err)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func sdkAccountAuth(initial accountInitial) icloud.AuthContext {
	headers := make([]icloud.Header, 0, len(initial.Headers))
	for name, value := range initial.Headers {
		headers = append(headers, icloud.Header{Name: name, Value: value})
	}

	auth := icloud.AuthContext{
		PhotosUploadServiceURL:    "",
		SharedPhotosServiceURL:    "",
		PhotosServiceURL:          "",
		LegacyRemindersServiceURL: "", RemindersServiceURL: "",
		DriveToken:       "",
		FindMyServiceURL: "", SetupServiceURL: "", SessionToken: nil,
		Cookies:                 sdkAccountCookies(initial.Cookies),
		DriveDocumentServiceURL: "",
		AccountID:               initial.Params[protocol.DSIDName], ClientID: initial.Params[protocol.ClientIDName],
		AccountServiceURL: initial.Origin, Headers: headers,
		ClientBuildNumber: nil, ClientMasteringNumber: nil, ChinaMainland: nil, DriveServiceURL: "",
	}

	if value, exists := initial.Params[protocol.ClientBuildNumberName]; exists {
		auth.ClientBuildNumber = &value
	}

	if value, exists := initial.Params[protocol.ClientMasteringNumberName]; exists {
		auth.ClientMasteringNumber = &value
	}

	if initial.China {
		region := true
		auth.ChinaMainland = &region
	}

	return auth
}

func assertDevicesSDKFailure(t *testing.T, scenario accountScenario,
	response *icloud.GetAccountDevicesResult, err error,
) {
	t.Helper()

	var failure *icloud.ClientError

	if response != nil || !errors.As(err, &failure) || failure.Kind() != sdkExpectedDeviceFailure(scenario) {
		t.Fatal("SDK provider failure lost its typed classification")
	}

	accountProviderFailure(t, scenario.Error, failure.StatusCode(), failure.ResponseBody())

	if strings.Contains(failure.Error(), replayLiteralSyntheticFailure) {
		t.Fatal("SDK display error disclosed provider content")
	}

	headers := failure.ResponseHeaders()
	if len(headers) != 1 ||
		headers[0].Name != protocol.HTTPContentTypeName || headers[0].Value != replayExpectedJSONMedia {
		t.Fatal("SDK provider failure lost headers")
	}
}

func assertDevicesSDKSuccess(t *testing.T, scenario accountScenario,
	response *icloud.GetAccountDevicesResult, err error,
) {
	t.Helper()

	if err != nil || response == nil {
		t.Fatalf("SDK device success: %v", err)
	}

	encoded, encodeErr := json.Marshal(response.Devices)
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}

	if !reflect.DeepEqual(accountJSON(t, encoded), accountJSON(t, scenario.Result)) {
		t.Fatalf("SDK device projection changed: %s", encoded)
	}

	if response.Metadata.StatusCode != scenario.Exchanges[0].Response.Status ||
		len(response.Metadata.Headers) != 1 || response.Metadata.Headers[0].Name != protocol.HTTPContentTypeName ||
		response.Metadata.Headers[0].Value != replayExpectedJSONMedia {
		t.Fatal("SDK device metadata changed")
	}
}

func sdkExpectedDeviceFailure(scenario accountScenario) icloud.ErrorKind {
	if scenario.Exchanges[0].Response.Status == 200 {
		return icloud.Provider
	}

	return icloud.Unavailable
}

func sdkAccountCookies(cookies []referenceCookie) []icloud.AuthCookie {
	result := make([]icloud.AuthCookie, 0, len(cookies))

	for _, cookie := range cookies {
		value := icloud.AuthCookie{Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain,
			HostOnly: false, Path: cookie.Path, Secure: cookie.Secure, HTTPOnly: false, Expires: nil, MaxAge: 0, SameSite: nil}

		if cookie.Expires != nil {
			expiry := time.Unix(*cookie.Expires, 0)
			value.Expires = &expiry
		}

		result = append(result, value)
	}

	return result
}
