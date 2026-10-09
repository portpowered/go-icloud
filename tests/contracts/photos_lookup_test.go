package contracts_test

import (
	"path/filepath"
	"testing"
)

func TestPhotoLookupWireContracts(t *testing.T) {
	t.Parallel()
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")

	paths, err := filepath.Glob("../replay/fixtures/synthetic/http/photos-get-*.json")
	if err != nil || len(paths) != 22 {
		t.Fatal("photo lookup contract inventory changed", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			validatePhotoAlbumExchanges(t, document, path)
		})
	}
}
