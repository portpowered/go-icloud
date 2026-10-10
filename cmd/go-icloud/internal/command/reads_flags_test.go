package command_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oapi-codegen/nullable"
	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

type legacyPhotoProbe struct {
	icloud.Client

	result *icloud.GetPhotoResult
}

func (probe legacyPhotoProbe) GetPhoto(_ context.Context, _ icloud.GetPhotoRequest) (*icloud.GetPhotoResult, error) {
	return probe.result, nil
}

func TestLegacyPhotoFlagsKeepOriginalOnlyInPrivateResult(t *testing.T) {
	t.Parallel()

	var resource icloud.PhotoResource

	resource.Url = json.RawMessage(`"https://assets.example.invalid/photo?token=synthetic-private-url"`)

	var photo icloud.Photo

	photo.ID = testSyntheticPhotoID
	photo.AssetMetadata = json.RawMessage(`{"unreviewed":"synthetic-private-metadata"}`)
	photo.Versions = map[string]icloud.PhotoResource{"original": resource}

	var result icloud.GetPhotoResult

	result.Photo = nullable.NewNullableWithValue(photo)
	result.Responses = []icloud.ResponseMetadata{}
	probe := legacyPhotoProbe{Client: nil, result: &result}
	directory := t.TempDir()
	session, saved := filepath.Join(directory, testSessionJsonFilename), filepath.Join(directory, testResultJsonFilename)
	writeFixtureValue(t, session, resultAuth())

	var output, diagnostic bytes.Buffer

	err := command.Run(t.Context(), probe,
		[]string{testSessionFlag, session, "--album", "Library", "--photo", testSyntheticPhotoID,
			testSaveResultFlag, saved, "photo"},
		&output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"synthetic-private-url", "synthetic-private-metadata",
		testVersionsKey, testAssetMetadataKey} {
		if strings.Contains(output.String(), marker) {
			t.Fatal("legacy flags exposed private provider data", marker)
		}
	}
	private, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(private, original) {
		t.Fatal("legacy explicit receipt differs from original result")
	}
}
