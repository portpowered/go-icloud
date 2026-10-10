package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

var errUnknownAccountOperation = errors.New("unsupported account SDK operation")

func TestRemainingAccountSDKPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(replayLiteralFixturesSyntheticHTTPAccountJSON)
	if err != nil {
		t.Fatal(err)
	}

	scenarios := 0

	for _, path := range paths {
		scenario := readAccountScenario(t, path)
		if scenario.Operation == replayDevicesOperation {
			continue
		}

		scenarios++

		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runAccountSDKService(t, scenario)
		})
	}

	if scenarios != 23 {
		t.Fatalf("remaining account SDK inventory changed: %d", scenarios)
	}
}

func runAccountSDKService(t *testing.T, scenario accountScenario) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	result, err := accountSDKService(t, client, scenario)
	if len(scenario.Error) != 0 {
		checkSDKServiceFailure(t, scenario, result, err)
	} else {
		if err != nil {
			t.Fatal(err)
		}

		checkSDKValue(t, result, scenario.Result)
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func accountSDKService(t *testing.T, client *icloud.SDK, scenario accountScenario) (any, error) {
	t.Helper()

	auth := sdkAccountAuth(scenario.Initial)

	switch scenario.Operation {
	case accountFamilyOperationName, replayLiteralFamilyPhotos:
		return accountSDKFamily(t, client, scenario, auth)
	case "storage":
		response, err := client.GetAccountStorage(t.Context(), icloud.GetAccountStorageRequest{Auth: auth})
		if err != nil {
			return nil, fmt.Errorf(replayAccountServiceErrorFormat, err)
		}

		checkSDKMetadata(t, response.Metadata, scenario.Exchanges[0].Response)
		checkSDKStorageProjection(t, response, scenario.Exchanges[0].Response)

		return map[string]int64{replayLiteralUsedBytes: response.Usage.UsedStorageInBytes,
			replayLiteralTotalBytes: response.Usage.TotalStorageInBytes}, nil
	case accountPlanOperationName:
		response, err := client.GetAccountPlanSummary(t.Context(), icloud.GetAccountPlanSummaryRequest{Auth: auth})
		if err != nil {
			return nil, fmt.Errorf(replayAccountServiceErrorFormat, err)
		}

		checkSDKMetadata(t, response.Metadata, scenario.Exchanges[0].Response)

		return response.Summary, nil
	default:
		t.Fatal("unsupported account SDK scenario")

		return nil, errUnknownAccountOperation
	}
}

func accountSDKFamily(t *testing.T, client *icloud.SDK, scenario accountScenario,
	auth icloud.AuthContext,
) (any, error) {
	t.Helper()

	response, err := client.GetAccountFamily(t.Context(), icloud.GetAccountFamilyRequest{Auth: auth})
	if err != nil {
		return nil, fmt.Errorf(replayAccountServiceErrorFormat, err)
	}

	checkSDKMetadata(t, response.Metadata, scenario.Exchanges[0].Response)
	checkSDKField(t, response.Members, scenario.Exchanges[0].Response, protocol.AccountFamilyResponseFamilyMembers)
	checkSDKFamilyMetadata(t, response, scenario.Exchanges[0].Response)

	if scenario.Operation == accountFamilyOperationName {
		names := make([]any, 0, len(response.Members))

		for _, member := range response.Members {
			if member.FullName.IsNull() || !member.FullName.IsSpecified() {
				names = append(names, nil)
			} else {
				names = append(names, member.FullName.MustGet())
			}
		}

		return names, nil
	}

	return accountSDKPhotos(t, client, scenario, auth, response.Members)
}

func accountSDKPhotos(t *testing.T, client *icloud.SDK, scenario accountScenario,
	auth icloud.AuthContext, members []icloud.AccountFamilyMember,
) (any, error) {
	t.Helper()

	photos := make([]*accountPhotoResult, 0, len(members))

	for index, member := range members {
		memberID := member.Dsid.MustGet()

		response, err := client.GetAccountMemberPhoto(t.Context(), icloud.GetAccountMemberPhotoRequest{
			Auth: auth, MemberID: memberID,
		})
		if err != nil {
			return nil, fmt.Errorf(replayAccountServiceErrorFormat, err)
		}

		checkSDKMetadata(t, response.Metadata, scenario.Exchanges[index+1].Response)

		headers := make([]replay.Pair, 0, len(response.Metadata.Headers))
		for _, header := range response.Metadata.Headers {
			headers = append(headers, replay.Pair{header.Name, header.Value})
		}

		photos = append(photos, &accountPhotoResult{MemberID: memberID, Status: response.Metadata.StatusCode,
			Headers: headers, Body: base64.StdEncoding.EncodeToString(response.Content)})
	}

	return photos, nil
}

func checkSDKServiceFailure(t *testing.T, scenario accountScenario, result any, err error) {
	t.Helper()

	var failure *icloud.ClientError

	if result != nil || !errors.As(err, &failure) || failure.Kind() != icloud.Unavailable {
		t.Fatalf("account SDK failure changed: %v", err)
	}

	accountProviderFailure(t, scenario.Error, failure.StatusCode(), failure.ResponseBody())
	checkSDKMetadata(t, icloud.ResponseMetadata{CookieScopeURL: failure.CookieScopeURL(), StatusCode: failure.StatusCode(),
		Headers: failure.ResponseHeaders()},
		scenario.Exchanges[len(scenario.Exchanges)-1].Response)

	if strings.Contains(failure.Error(), replayLiteralSyntheticFailure) {
		t.Fatal("provider content leaked into the display error")
	}
}

func checkSDKMetadata(t *testing.T, actual icloud.ResponseMetadata, expected *replay.Response) {
	t.Helper()

	headers := make([]replay.Pair, 0, len(actual.Headers))
	for _, header := range actual.Headers {
		headers = append(headers, replay.Pair{header.Name, header.Value})
	}

	if actual.StatusCode != expected.Status || !reflect.DeepEqual(headers, expected.Headers) {
		t.Fatal("public HTTP metadata changed")
	}
}

func checkSDKValue(t *testing.T, actual any, expected json.RawMessage) {
	t.Helper()

	encoded, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(accountJSON(t, encoded), accountJSON(t, expected)) {
		t.Fatalf("public account projection changed: %s", encoded)
	}
}

func checkSDKFamilyMetadata(t *testing.T, response *icloud.GetAccountFamilyResult, expected *replay.Response) {
	t.Helper()

	fields := sdkResponseFields(t, expected)
	delete(fields, protocol.AccountFamilyResponseFamilyMembers)

	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, response.AdditionalMetadata, encoded)
}

func checkSDKStorageProjection(t *testing.T, response *icloud.GetAccountStorageResult, expected *replay.Response) {
	t.Helper()

	checkSDKField(t, response.Usage, expected, protocol.AccountStorageResponseStorageUsageInfo)
	checkSDKField(t, response.Media, expected, protocol.AccountStorageResponseStorageUsageByMedia)

	fields := sdkResponseFields(t, expected)
	if quota, exists := fields[protocol.AccountStorageResponseQuotaStatus]; exists {
		checkSDKValue(t, response.Quota, quota)
	} else if response.Quota != nil {
		t.Fatal("omitted quota metadata was invented")
	}

	delete(fields, protocol.AccountStorageResponseStorageUsageInfo)
	delete(fields, protocol.AccountStorageResponseStorageUsageByMedia)
	delete(fields, protocol.AccountStorageResponseQuotaStatus)

	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}

	checkSDKValue(t, response.AdditionalMetadata, encoded)
}

func sdkResponseFields(t *testing.T, response *replay.Response) map[string]json.RawMessage {
	t.Helper()

	encoded, ok := accountJSON(t, response.Body.Value).(string)
	if !ok || response.Body.Encoding != testBase64Encoding {
		t.Fatal("expected an exact-byte account response fixture")
	}

	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage

	err = json.Unmarshal(body, &fields)
	if err != nil {
		t.Fatal(err)
	}

	return fields
}

func checkSDKField(t *testing.T, actual any, response *replay.Response, key string) {
	t.Helper()

	fields := sdkResponseFields(t, response)

	expected := fields[key]
	if len(expected) == 0 {
		expected = json.RawMessage("[]")
	}

	checkSDKValue(t, actual, expected)
}
