package contracts_test

import (
	"path/filepath"
	"testing"
)

func TestRecentlyAddedPhotosWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-recently-added-*.json")
	if err != nil || len(paths) != 16 {
		t.Fatal("recently added contract inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			validatePhotoAlbumExchanges(t, document, path)
		})
	}
}
