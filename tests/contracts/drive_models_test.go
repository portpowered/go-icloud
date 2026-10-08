package contracts_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

func TestDriveModelExamples(t *testing.T) {
	t.Parallel()

	document := loadDriveDocument(t, driveModelsPath)
	examples := 0

	for name, schema := range document.Components.Schemas {
		if schema.Value.Example == nil {
			continue
		}

		err := schema.Value.VisitJSON(schema.Value.Example)
		if err != nil {
			t.Fatalf("%s example: %v", name, err)
		}

		examples++
	}

	if examples != 4 {
		t.Fatalf("Drive example inventory changed: %d", examples)
	}
}

func TestDriveModelsPreserveUnknownValuesAndPresence(t *testing.T) {
	t.Parallel()

	for name, target := range map[string]any{
		"node":           new(drive.DriveNode),
		"nullable-error": new(drive.DriveError),
		"token":          new(drive.DriveDownloadTokens),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			input := `{"future":{"large":9007199254740993,"nil":null},"futureArray":[null,false,9007199254740993]}`

			switch name {
			case "node":
				input = `{"size":"0","type":"FUTURE_KIND","status":"FUTURE_STATUS",` +
					`"shareID":{"future":null},"items":[],"future":9007199254740993}`
			case "nullable-error":
				input = `{"reason":null,"error":null,"errorCode":null,"future":null}`
			}

			assertDriveRoundTrip(t, input, target)
		})
	}
}

func TestDriveWireDatesRetainNegativeMinuteOffsets(t *testing.T) {
	t.Parallel()

	var node drive.DriveNode

	input := `{"lastOpenTime":"2024-01-02T03:04:05-07:30"}`
	assertDriveRoundTrip(t, input, &node)

	if node.LastOpenTime == nil || node.LastOpenTime.UTC().Format("2006-01-02T15:04:05Z") != "2024-01-02T10:34:05Z" {
		t.Fatal("negative offset minutes were lost")
	}
}

func TestDriveTemporaryFolderIDsRequireUUIDv4(t *testing.T) {
	t.Parallel()

	document := loadDriveDocument(t, driveModelsPath)
	schema := document.Components.Schemas["DriveFolderCreation"].Value

	for _, identifier := range []string{
		"FOLDER::UNKNOWN_ZONE::TempId-00000000-0000-1000-8000-000000000000",
		"FOLDER::UNKNOWN_ZONE::TempId-00000000-0000-4000-0000-000000000000",
		"FOLDER::UNKNOWN_ZONE::TempId-AAAAAAAA-0000-4000-8000-000000000000",
	} {
		value := map[string]any{contractClientIDKey: identifier, "name": "synthetic"}
		if schema.VisitJSON(value) == nil {
			t.Fatalf("non-reference temporary identifier accepted: %s", identifier)
		}
	}

	value := map[string]any{contractClientIDKey: "FOLDER::UNKNOWN_ZONE::TempId-aaaaaaaa-0000-4000-8000-000000000000",
		"name": "synthetic"}

	err := schema.VisitJSON(value)
	if err != nil {
		t.Fatalf("valid UUIDv4 rejected: %v", err)
	}
}

func assertDriveRoundTrip(t *testing.T, input string, target any) {
	t.Helper()

	err := json.Unmarshal([]byte(input), target)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(target)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(drivePreciseJSON(t, []byte(input)), drivePreciseJSON(t, encoded)) {
		t.Fatalf("Drive model changed metadata or presence: %s", encoded)
	}
}

func drivePreciseJSON(t *testing.T, data []byte) any {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var value any

	err := decoder.Decode(&value)
	if err != nil {
		t.Fatal(err)
	}

	return value
}
