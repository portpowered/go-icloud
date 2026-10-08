package replay_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/internal/accountapi"
	"github.com/portpowered/go-icloud/tests/replay"
)

const accountFamilyOperationName = "family"

type accountScenario struct {
	Operation string `json:"operation"`
	//nolint:tagliatelle // LIB-05: the portable fixture fixes this field spelling.
	Initial   accountInitial    `json:"initial_state"`
	Exchanges []replay.Exchange `json:"exchanges"`
	Result    json.RawMessage   `json:"result"`
	Error     json.RawMessage   `json:"error"`
}

type accountInitial struct {
	Origin  string            `json:"origin"`
	Params  map[string]string `json:"params"`
	Headers map[string]string `json:"headers"`
	//nolint:tagliatelle // LIB-05: the portable fixture fixes this field spelling.
	China bool `json:"china_mainland"`
}

func TestGeneratedAccountClientPortableScenarios(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob("fixtures/synthetic/http/account-*.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) != 26 {
		t.Fatal("account scenario inventory changed")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			runGeneratedAccount(t, readAccountScenario(t, path))
		})
	}
}

func readAccountScenario(t *testing.T, path string) accountScenario {
	t.Helper()

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}

	var scenario accountScenario

	err = json.Unmarshal(data, &scenario)
	if err != nil {
		t.Fatal(err)
	}

	return scenario
}

func accountParameters[T any](t *testing.T, initial accountInitial) *T {
	t.Helper()

	values := make(map[string]string)
	maps.Copy(values, initial.Params)
	maps.Copy(values, initial.Headers)

	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}

	parameters := new(T)

	err = json.Unmarshal(data, parameters)
	if err != nil {
		t.Fatal(err)
	}

	return parameters
}

func runGeneratedAccount(t *testing.T, scenario accountScenario) {
	t.Helper()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	doer := new(http.Client)
	doer.Transport = transport

	origin := scenario.Initial.Origin
	if scenario.Operation == "summary_plan" {
		origin = "https://gatewayws.icloud.com"
		if scenario.Initial.China {
			origin += ".cn"
		}
	}

	client, err := accountapi.NewClientWithResponses(origin, accountapi.WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}

	result := accountOperation(t, client, scenario)
	if len(scenario.Error) == 0 {
		actual, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}

		if !reflect.DeepEqual(accountJSON(t, actual), accountJSON(t, scenario.Result)) {
			t.Fatalf("account result changed: %s", actual)
		}
	}

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func accountJSON(t *testing.T, data []byte) any {
	t.Helper()

	var value any

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	err := decoder.Decode(&value)
	if err != nil {
		t.Fatal(err)
	}

	return value
}

func accountOperation(t *testing.T, client *accountapi.ClientWithResponses, scenario accountScenario) any {
	t.Helper()

	switch scenario.Operation {
	case "devices":
		return accountDevicesOperation(t, client, scenario)
	case accountFamilyOperationName, "family_photos":
		return accountFamilyOperation(t, client, scenario)
	case "storage":
		return accountStorageOperation(t, client, scenario)
	case "summary_plan":
		return accountPlanOperation(t, client, scenario)
	default:
		t.Fatalf("unimplemented account operation: %s", scenario.Operation)

		return nil
	}
}

func accountFamilyOperation(t *testing.T, client *accountapi.ClientWithResponses, scenario accountScenario) any {
	t.Helper()

	response, err := client.ListAccountFamilyWithResponse(t.Context(),
		accountParameters[accountapi.ListAccountFamilyParams](t, scenario.Initial))
	if err != nil || response == nil || response.JSON200 == nil {
		t.Fatalf("account family: %v", err)
	}

	names := make([]string, 0)
	photos := make([]*accountPhotoResult, 0)

	if response.JSON200.FamilyMembers == nil {
		return names
	}

	for _, member := range *response.JSON200.FamilyMembers {
		if scenario.Operation == accountFamilyOperationName {
			name, nameErr := member.FullName.Get()
			if nameErr != nil {
				t.Fatal(nameErr)
			}

			names = append(names, name)
		} else {
			photo := accountMemberPhoto(t, client, scenario, member.Dsid.MustGet())
			if photo == nil {
				return nil
			}

			photos = append(photos, photo)
		}
	}

	if scenario.Operation == accountFamilyOperationName {
		return names
	}

	return photos
}

func accountMemberPhoto(t *testing.T, client *accountapi.ClientWithResponses,
	scenario accountScenario, memberID string,
) *accountPhotoResult {
	t.Helper()

	parameters := accountParameters[accountapi.GetFamilyMemberPhotoParams](t, scenario.Initial)
	parameters.MemberId = memberID

	response, err := accountapi.ReadFamilyMemberPhoto(t.Context(), client, parameters)
	if err != nil || response == nil {
		t.Fatalf("member photo: %v", err)
	}

	if len(scenario.Error) != 0 {
		if response.Status != http.StatusServiceUnavailable {
			t.Fatal("member photo lost provider failure")
		}

		accountProviderFailure(t, scenario.Error, response.Status, response.Body)

		return nil
	}

	return &accountPhotoResult{
		MemberID: memberID, Status: response.Status,
		Headers: []replay.Pair{{"Content-Type", response.Headers.Get("Content-Type")}},
		Body:    base64.StdEncoding.EncodeToString(response.Body),
	}
}

type accountPhotoResult struct {
	//nolint:tagliatelle // LIB-05: the portable result fixes this field spelling.
	MemberID string        `json:"member_id"`
	Status   int           `json:"status"`
	Headers  []replay.Pair `json:"headers"`
	Body     string        `json:"body"`
}

func accountDevicesOperation(t *testing.T, client *accountapi.ClientWithResponses, scenario accountScenario) any {
	t.Helper()

	response, err := client.ListAccountDevicesWithResponse(t.Context(),
		accountParameters[accountapi.ListAccountDevicesParams](t, scenario.Initial))
	if err != nil || response == nil {
		t.Fatalf("account devices: %v", err)
	}

	if len(scenario.Error) != 0 {
		if response.StatusCode() != scenario.Exchanges[0].Response.Status ||
			(response.JSONDefault == nil && response.JSON200 == nil) {
			t.Fatal("account devices lost provider failure")
		}

		accountProviderFailure(t, scenario.Error, response.StatusCode(), response.Body)

		return nil
	}

	if response.JSON200 == nil {
		t.Fatal("account devices has no decoded success")
	}

	return response.JSON200.Devices
}

func accountStorageOperation(t *testing.T, client *accountapi.ClientWithResponses, scenario accountScenario) any {
	t.Helper()

	response, err := client.GetAccountStorageWithResponse(t.Context(),
		accountParameters[accountapi.GetAccountStorageParams](t, scenario.Initial))
	if err != nil || response == nil || response.JSON200 == nil {
		t.Fatalf("account storage: %v", err)
	}

	return map[string]int64{
		"used_bytes":  response.JSON200.StorageUsageInfo.UsedStorageInBytes,
		"total_bytes": response.JSON200.StorageUsageInfo.TotalStorageInBytes,
	}
}

func accountPlanOperation(t *testing.T, client *accountapi.ClientWithResponses, scenario accountScenario) any {
	t.Helper()

	response, err := client.GetAccountPlanSummaryWithResponse(t.Context(), scenario.Initial.Params["dsid"],
		accountParameters[accountapi.GetAccountPlanSummaryParams](t, scenario.Initial))
	if err != nil || response == nil {
		t.Fatalf("account plan: %v", err)
	}

	if len(scenario.Error) != 0 {
		if response.StatusCode() != http.StatusServiceUnavailable || response.JSONDefault == nil {
			t.Fatal("account plan lost provider failure")
		}

		accountProviderFailure(t, scenario.Error, response.StatusCode(), response.Body)

		return nil
	}

	return response.JSON200
}

type accountFailureExpectation struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func accountProviderFailure(t *testing.T, encoded json.RawMessage, status int, body []byte) {
	t.Helper()

	var expected accountFailureExpectation

	err := json.Unmarshal(encoded, &expected)
	if err != nil {
		t.Fatal(err)
	}

	if expected.Type != "PyiCloudAPIResponseException" ||
		expected.Message != accountProviderMessage(t, status, body) {
		t.Fatal("account provider failure lost its recorded class/status/body")
	}
}

func accountProviderMessage(t *testing.T, status int, body []byte) string {
	t.Helper()

	if status != http.StatusOK {
		return fmt.Sprintf(" (%d): %s", status, body)
	}

	fields, ok := accountJSON(t, body).(map[string]any)
	if !ok {
		t.Fatal("provider envelope is not an object")
	}

	reason := "Unknown reason"

	for _, key := range []string{"errorMessage", "reason", "errorReason", "error"} {
		if text, matches := fields[key].(string); matches && text != "" {
			reason = text

			break
		}
	}

	return reason + ": " + string(body)
}
