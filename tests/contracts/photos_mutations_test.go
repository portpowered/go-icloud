package contracts_test

import (
	"github.com/getkin/kin-openapi/openapi3"
	"testing"
)

func TestPhotoMutationVariantsRejectMalformedRequests(t *testing.T) {
	t.Parallel()
	models := loadDriveDocument(t, "../../api/external/photos-mutations-models.openapi.yaml")
	document := loadDriveDocument(t, "../../api/external/photos.openapi.yaml")

	for variant, names := range map[string][]string{
		"PhotoAlbumCreationRequest": {"photos-create-album", "photos-create-folder",
			"photos-shared-library-create-album", "photos-shared-library-create-album-refused-4"},
		"PhotoAlbumRenameRequest":   {"photos-rename-album"},
		"PhotoAlbumDeletionRequest": {"photos-delete-album"},
		"PhotoAlbumRelationRequest": {"photos-add-to-album", "photos-add-to-empty-album"},
		"PhotoFavoriteRequest":      {"photos-favorite-true", "photos-favorite-false", "photos-shared-library-favorite-true"},
		"PhotoAssetDeletionRequest": {"photos-delete-asset"},
	} {
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				exchanges := accountExchanges(t, "../replay/fixtures/synthetic/http/"+name+".json")
				for _, pair := range exchanges {
					if pair.Request.Path != "/database/1/com.apple.photos.cloud/production/private/records/modify" &&
						pair.Request.Path != "/database/1/com.apple.photos.cloud/production/shared/records/modify" {
						continue
					}

					value, err := driveJSONValue(pair.Request.Body)
					if err != nil {
						t.Fatal(err)
					}

					for _, schema := range []*openapi3.Schema{models.Components.Schemas[variant].Value,
						models.Components.Schemas["PhotoMutationRequest"].Value,
						document.Paths.Value(pair.Request.Path).Post.RequestBody.Value.Content["application/json"].Schema.Value} {
						err = schema.VisitJSON(value)
						if err != nil {
							t.Fatal(err)
						}

						checkPhotoMutationNegatives(t, schema, value)
					}
				}
			})
		}
	}
}
func checkPhotoMutationNegatives(t *testing.T, schema *openapi3.Schema, value any) {
	t.Helper()

	fields := reminderWriteFields(t, value)
	for key, original := range fields {
		delete(fields, key)

		if schema.VisitJSON(value) == nil {
			t.Fatalf("required field %s deletion accepted", key)
		}

		fields[key] = map[string]any{photoMutationWrapperType: "INVALID", "value": true}

		if schema.VisitJSON(value) == nil {
			t.Fatalf("invalid field %s accepted", key)
		}

		fields[key] = original
	}
}

const photoMutationWrapperType = "type"
