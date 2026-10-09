package contracts_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/tests/replay"
)

const (
	remindersSchemaPath = "../../api/external/reminders.openapi.yaml"
	cloudKitModelsPath  = "../../api/external/cloudkit-models.openapi.yaml"
)

type remindersContractInventory struct {
	Scenarios  int
	Pairs      int
	Operations int
	Invalid    int
}

var errRemindersBinding = errors.New("unbound Reminders operation")

func TestPortableRemindersWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/reminders-*.json")
	if err != nil {
		t.Fatal(err)
	}

	pairs, invalid := 0, 0
	operations := make(map[string]int)

	for _, path := range paths {
		exchanges := accountExchanges(t, path)
		for index, exchange := range exchanges {
			operation, err := bindRemindersOperation(document, exchange.Request, exchanges[:index])
			if err != nil {
				t.Fatalf("%s exchange%d: %v", path, index, err)
			}

			invalidResponse := invalidReminderResponse(filepath.Base(path), index)
			validateRemindersExchange(t, operation, exchange, invalidResponse)

			if invalidResponse {
				invalid++
			}

			pairs++
			operations[operation.OperationID]++
		}
	}

	got := remindersContractInventory{Scenarios: len(paths), Pairs: pairs, Operations: len(operations), Invalid: invalid}

	want := remindersContractInventory{Scenarios: 357, Pairs: 423, Operations: 7, Invalid: 51}
	if got != want {
		t.Fatalf("Reminders contract inventory changed: %+v", got)
	}
}

func invalidReminderResponse(name string, index int) bool {
	if strings.HasPrefix(name, "reminders-related-") {
		return invalidReminderRelatedResponse(name, index)
	}

	if invalidReminderUnionResponse(name) {
		return index == 0
	}

	switch name {
	case "reminders-changes-invalid-json.json", "reminders-changes-invalid-zone.json",
		"reminders-changes-invalid-record.json", "reminders-sync-invalid-json-fallback.json",
		"reminders-sync-schema-query-fallback.json", "reminders-sync-schema-record-name.json",
		"reminders-sync-schema-record-fields.json":
		return index == 0
	case "reminders-sync-schema-zone.json":
		return index == 1
	default:
		if strings.HasPrefix(name, "reminders-sync-wrapper-") && strings.HasSuffix(name, "-invalid.json") {
			return index == 0
		}

		return invalidReminderRequiredFields(name)
	}
}

func invalidReminderRequiredFields(name string) bool {
	return name == "reminders-legacy-missing-lists.json" || name == "reminders-zones-schema-error.json" ||
		strings.HasPrefix(name, "reminders-lists-schema-") ||
		strings.HasPrefix(name, "reminders-get-schema-") || name == "reminders-get-no-records.json"
}

func invalidReminderRelatedResponse(name string, index int) bool {
	return (strings.Contains(name, "wire-invalid") || strings.Contains(name, "validation-before-error")) &&
		(!strings.Contains(name, "alarms-trigger-") || index == 1)
}

func invalidReminderUnionResponse(name string) bool {
	return (strings.HasPrefix(name, "reminders-list-union-") || strings.HasPrefix(name, "reminders-snapshot-union-")) &&
		(strings.HasSuffix(name, "-wire-validation.json") || strings.HasSuffix(name, "-zone-validation.json"))
}

func bindRemindersOperation(document *openapi3.T, request replay.Request,
	preceding []replay.Exchange,
) (*openapi3.Operation, error) {
	item := document.Paths.Value(request.Path)
	if item != nil {
		if operation := item.GetOperation(request.Method); operation != nil {
			return operation, nil
		}
	}

	if request.Method == http.MethodGet && issuedRemindersAsset(preceding, request) {
		return document.Paths.Value("/{assetPath}").Get, nil
	}

	return nil, errRemindersBinding
}

func remindersRequestTarget(operation string) any {
	switch operation {
	case "RemindersLookupRecords":
		return new(cloudkit.CKLookupRequest)
	case "RemindersQueryRecords":
		return new(cloudkit.CKQueryRequest)
	case "RemindersModifyRecords":
		return new(cloudkit.CKModifyRequest)
	case "RemindersZoneChanges":
		return new(cloudkit.CKZoneChangesRequest)
	case "RemindersListZones":
		return new(cloudkit.CKEmptyRequest)
	default:
		panic("unknown Reminders request model")
	}
}

func remindersReplyTarget(operation string) any {
	switch operation {
	case "RemindersLookupRecords":
		return new(cloudkit.CKLookupResponse)
	case "RemindersQueryRecords":
		return new(cloudkit.CKQueryResponse)
	case "RemindersModifyRecords":
		return new(cloudkit.CKModifyResponse)
	case "RemindersZoneChanges":
		return new(cloudkit.CKZoneChangesResponse)
	case "RemindersListZones":
		return new(cloudkit.CKZoneListResponse)
	default:
		panic("unknown Reminders reply model")
	}
}

func TestRemindersGenerationHasNoDrift(t *testing.T) {
	t.Parallel()

	for _, artifact := range []generationArtifact{
		{Schema: cloudKitModelsPath, Config: "../../pkg/dependencymodels/cloudkit/config.yaml",
			Output: "../../pkg/dependencymodels/cloudkit/models.gen.go"},
		{Schema: remindersSchemaPath, Config: "../../internal/remindersapi/config.yaml",
			Output: "../../internal/remindersapi/client.gen.go"},
	} {
		t.Run(filepath.Base(artifact.Output), func(t *testing.T) { t.Parallel(); verifyGeneration(t, artifact) })
	}
}

func TestCloudKitModelsRetainExtensibleRecordFields(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		`{"recordName":"Reminder/synthetic","recordType":"Reminder","fields":{},"pluginFields":{},"future":9007199254740993}`,
		`{"recordName":"Reminder/synthetic","recordType":"Reminder","fields":` +
			`{"Future":{"type":"FUTURE","value":[null,false,9007199254740993]}},` +
			`"modified":null,"deleted":false,"expirationTime":null}`,
		`{"recordName":"Reminder/synthetic","recordType":"Reminder","fields":` +
			`{"TitleEncrypted":{"type":"ENCRYPTED_BYTES","value":"c3ludGhldGlj"}},` +
			`"recordChangeTag":null,"zoneID":null}`,
	} {
		assertDriveRoundTrip(t, input, new(cloudkit.CKRecord))
	}
}

func TestRemindersBindingsRejectUnknownPostRoutes(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, remindersSchemaPath)
	request := new(replay.Request)
	request.Method = http.MethodPost
	request.Path = "/unknown"

	_, err := bindRemindersOperation(document, *request, nil)
	if !errors.Is(err, errRemindersBinding) {
		t.Fatalf("unknown route accepted: %v", err)
	}
}

func TestCloudKitSchemaRejectsMissingRecordIdentity(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, cloudKitModelsPath)

	for name, input := range map[string]string{
		"CKRecord":          `{"recordName":"synthetic"}`,
		"CKLookupRequest":   `{"records":[]}`,
		"CKModifyOperation": `{"operationType":"invalid","record":{"recordName":"synthetic","recordType":"Reminder"}}`,
		"CKZoneIDReq":       `{"zoneName":1}`,
	} {
		var value any

		err := json.Unmarshal([]byte(input), &value)
		if err != nil {
			t.Fatal(err)
		}

		if document.Components.Schemas[name].Value.VisitJSON(value) == nil {
			t.Fatalf("invalid %s accepted", name)
		}
	}
}

func issuedRemindersAsset(preceding []replay.Exchange, request replay.Request) bool {
	for _, exchange := range preceding {
		if exchange.Request.Method != http.MethodPost || exchange.Response == nil ||
			exchange.Response.Status >= http.StatusBadRequest {
			continue
		}

		for _, record := range reminderIssuedRecords(exchange.Response.Body) {
			address := reminderAssetRecordURL(record)
			if reminderAssetURLMatches(address, request) {
				return true
			}
		}
	}

	return false
}

func reminderIssuedRecords(body replay.Entity) []any {
	value, err := driveJSONValue(body)
	if err != nil {
		return nil
	}

	object, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	records, _ := object["records"].([]any)

	zones, _ := object["zones"].([]any)
	for _, zone := range zones {
		if item, ok := zone.(map[string]any); ok {
			list, _ := item["records"].([]any)
			records = append(records, list...)
		}
	}

	return records
}

func reminderAssetRecordURL(value any) string {
	record, valid := value.(map[string]any)
	if !valid || record["recordType"] != "List" {
		return ""
	}

	fields, valid := record["fields"].(map[string]any)
	if !valid {
		return ""
	}

	field, valid := fields["ReminderIDsAsset"].(map[string]any)
	if !valid || field["type"] != "ASSETID" {
		return ""
	}

	asset, valid := field["value"].(map[string]any)
	if !valid {
		return ""
	}

	address, _ := asset["downloadURL"].(string)

	return address
}

func reminderAssetURLMatches(address string, request replay.Request) bool {
	target, err := url.Parse(address)
	if err != nil {
		return false
	}

	if target.Scheme != "https" || target.User != nil || target.Host == "" {
		return false
	}

	query := make(url.Values)
	for _, pair := range request.Query {
		query.Add(pair[0], pair[1])
	}

	return target.Scheme+"://"+target.Host == request.Origin && target.EscapedPath() == request.Path &&
		reflect.DeepEqual(target.Query(), query)
}

func TestRemindersAssetBindingRejectsUnissuedRequests(t *testing.T) {
	t.Parallel()

	document := loadDriveDocument(t, remindersSchemaPath)

	for _, mutation := range []string{"asset-origin", mutationPath, "asset-query", "orphan", mutationMethod} {
		t.Run(mutation, func(t *testing.T) {
			t.Parallel()
			exchanges := accountExchanges(t, "../replay/fixtures/synthetic/http/reminders-lists-asset-membership-1.json")
			request := exchanges[1].Request
			preceding := exchanges[:1]

			switch mutation {
			case "asset-origin":
				request.Origin = "https://unissued.example.invalid"
			case mutationPath:
				request.Path = "/unissued.json"
			case "asset-query":
				request.Query = []replay.Pair{{"unissued", "value"}}
			case "orphan":
				preceding = nil
			case mutationMethod:
				request.Method = http.MethodPost
			}

			_, err := bindRemindersOperation(document, request, preceding)
			if !errors.Is(err, errRemindersBinding) {
				t.Fatal("unissued asset accepted")
			}
		})
	}
}

func TestCloudKitModelExamplesValidate(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, cloudKitModelsPath)
	count := 0

	for name, schema := range document.Components.Schemas {
		if schema.Value.Example == nil {
			continue
		}

		err := schema.Value.VisitJSON(schema.Value.Example)
		if err != nil {
			t.Fatalf("%s example: %v", name, err)
		}

		count++
	}

	if count != 16 {
		t.Fatalf("CloudKit example inventory changed: %d", count)
	}
}

func validateRemindersExchange(t *testing.T, operation *openapi3.Operation, exchange replay.Exchange,
	invalidResponse bool,
) {
	t.Helper()

	if operation.OperationID == "RemindersLegacyStartup" {
		validateFindMyParameters(t, operation, exchange.Request)
	}

	if exchange.Request.Method == http.MethodPost {
		validateFindMyParameters(t, operation, exchange.Request)
		validateDriveRequest(t, operation, exchange.Request)
		assertDriveRoundTrip(t, string(findMyEntityBytes(t, exchange.Request.Body)),
			remindersRequestTarget(operation.OperationID))
	}

	if invalidResponse {
		response := driveResponseContract(operation, exchange.Response.Status)

		value, err := driveJSONValue(exchange.Response.Body)
		if err != nil {
			return
		}

		if response.Value.Content["application/json"].Schema.Value.VisitJSON(value) == nil {
			t.Fatal("Source-invalid zone payload accepted by the canonical schema")
		}

		return
	}

	validateDriveResponse(t, operation, exchange.Response)

	if exchange.Request.Method == http.MethodPost && exchange.Response.Status < http.StatusBadRequest {
		assertDriveRoundTrip(t, string(findMyEntityBytes(t, exchange.Response.Body)),
			remindersReplyTarget(operation.OperationID))
	}
}

func TestCloudKitNormalRecordsAreExplicitResponseMembers(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, cloudKitModelsPath)
	value := map[string]any{"recordName": "Reminder/synthetic", "recordType": "Reminder", "fields": map[string]any{}}

	for _, name := range []string{"CKLookupResponse", "CKModifyResponse", "CKQueryResponse", "CKZoneChangesZone"} {
		item := document.Components.Schemas[name].Value.Properties["records"].Value.Items.Value

		err := item.VisitJSON(value)
		if err != nil {
			t.Fatalf("%s omitted normal records: %v", name, err)
		}

		kept := make(openapi3.SchemaRefs, 0, len(item.AnyOf))

		for _, member := range item.AnyOf {
			if member.Ref != "#/components/schemas/CKRecord" {
				kept = append(kept, member)
			}
		}

		if len(kept) == len(item.AnyOf) {
			t.Fatal("normal record schema member missing")
		}

		item.AnyOf = kept
		if item.VisitJSON(value) == nil {
			t.Fatal("normal records bypassed their required schema member")
		}
	}
}
