package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	testFolderPrefix = "FOLDER::SYNTHETIC_ZONE::TempId-"
	//nolint:gosec // GO-15: this is a synthetic cookie header name, not a credential.
	testUploadCookie       = "X-SYNTHETIC-COOKIE"
	testUploadTokenPattern = `\bt=([^;]+)`
)

func TestAccountProtocolConstantsHaveNoDrift(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile("../../api/external/account.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	err = document.Validate(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	actual, err := constantsSource(document)
	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("../../internal/protocol/account.gen.go")
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("protocol constants drifted; run make generate-api")
	}
}

func TestAccountProtocolConstantsRejectAmbiguousIdentifiers(t *testing.T) {
	t.Parallel()

	for _, operationID := range []string{"not-an-identifier", "same"} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()

			loader := openapi3.NewLoader()
			loader.IsExternalRefsAllowed = true

			document, err := loader.LoadFromFile("../../api/external/account.openapi.yaml")
			if err != nil {
				t.Fatal(err)
			}

			for _, item := range document.Paths.Map() {
				for _, operation := range item.Operations() {
					operation.OperationID = operationID
				}
			}

			_, err = constantsSource(document)
			if !errors.Is(err, errConstant) {
				t.Fatal("invalid or duplicate operation constants were emitted")
			}
		})
	}
}

func TestAccountProtocolHeaderNamesComeFromSchema(t *testing.T) {
	t.Parallel()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile("../../api/external/account.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	header := document.Components.Headers["Content-Type"]
	delete(document.Components.Headers, "Content-Type")
	document.Components.Headers["X-Synthetic-Media"] = header

	values, err := accountConstants(document)
	if err != nil {
		t.Fatal(err)
	}

	if _, exists := values["HTTPContentTypeName"]; exists || values["HTTPXSyntheticMediaName"] != "X-Synthetic-Media" {
		t.Fatal("header constants did not follow their canonical schema declaration")
	}
}

func TestDriveProtocolConstantsHaveNoDrift(t *testing.T) {
	t.Parallel()

	document := driveConstantsDocument(t)

	values, err := externalConstants(document, driveModelsDocument(t))
	if err != nil {
		t.Fatal(err)
	}

	actual, err := renderConstants(values, "Drive")
	if err != nil {
		t.Fatal(err)
	}

	expected, err := os.ReadFile("../../internal/protocol/drive.gen.go")
	if err != nil || !bytes.Equal(actual, expected) {
		t.Fatal("Drive constants drifted; run make generate-api")
	}
}

func TestDriveProtocolConstantsFollowSchemaAndRejectCollisions(t *testing.T) {
	t.Parallel()

	document := driveConstantsDocument(t)
	models := driveModelsDocument(t)
	node := models.Components.Schemas["DriveAppLibraries"].Value
	node.Properties["futureItems"] = node.Properties["items"]
	delete(node.Properties, "items")

	values, err := externalConstants(document, models)
	if err != nil || values["DriveAppLibrariesFutureItems"] != "futureItems" {
		t.Fatal("Drive constants did not follow the schema property")
	}

	if _, exists := values["DriveAppLibrariesItems"]; exists {
		t.Fatal("obsolete Drive property constant survived")
	}

	for _, item := range document.Paths.Map() {
		for _, operation := range item.Operations() {
			operation.OperationID = "DriveUpdateDocument"
		}
	}

	_, err = externalConstants(document, models)
	if !errors.Is(err, errConstant) {
		t.Fatal("ambiguous Drive operation/model constant was accepted")
	}
}

func driveConstantsDocument(t *testing.T) *openapi3.T {
	t.Helper()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile("../../api/external/drive.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	return document
}

func driveModelsDocument(t *testing.T) *openapi3.T {
	t.Helper()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile("../../api/external/drive-models.openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	return document
}

func TestDrivePrefixConstantsRejectTruncationAndInvalidDeclarations(t *testing.T) {
	t.Parallel()

	for _, value := range []any{"FOLDER", "", "OTHER::", 1} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			t.Parallel()

			models := driveModelsDocument(t)
			field := models.Components.Schemas["DriveFolderCreation"].Value.Properties["clientId"].Value
			field.Extensions["x-protocol-prefix"] = value

			_, err := externalConstants(driveConstantsDocument(t), models)
			if !errors.Is(err, errConstant) {
				t.Fatal("invalid or truncated identifier prefix was emitted")
			}
		})
	}
}

func TestDrivePrefixConstantsFollowCompletePatternAndExtension(t *testing.T) {
	t.Parallel()

	models := driveModelsDocument(t)
	field := models.Components.Schemas["DriveFolderCreation"].Value.Properties["clientId"].Value
	field.Pattern = strings.ReplaceAll(field.Pattern, "UNKNOWN_ZONE", "SYNTHETIC_ZONE")
	field.Extensions["x-protocol-prefix"] = testFolderPrefix

	values, err := externalConstants(driveConstantsDocument(t), models)
	if err != nil || values["DriveFolderCreationClientIdPrefix"] != testFolderPrefix {
		t.Fatal("prefix did not follow the canonical model declaration")
	}

	field.Pattern = strings.TrimPrefix(field.Pattern, "^")

	_, err = externalConstants(driveConstantsDocument(t), models)
	if !errors.Is(err, errConstant) {
		t.Fatal("unanchored temporary identifier prefix was emitted")
	}
}

func TestDriveRequestMediaConstantsFollowSchema(t *testing.T) {
	t.Parallel()

	document := driveConstantsDocument(t)
	content := document.Paths.Value("/createFolders").Post.RequestBody.Value.Content
	content["application/x-synthetic"] = content["plain/text"]
	delete(content, "plain/text")

	values, err := externalConstants(document, driveModelsDocument(t))
	if err != nil || values["MediaApplicationXSynthetic"] != "application/x-synthetic" {
		t.Fatal("request media type did not follow the route declaration")
	}

	if _, exists := values["MediaPlainText"]; !exists {
		// The upload and registration routes still own this request media type.
		t.Fatal("other route-owned plain/text media was lost")
	}
}

func TestDriveScalarConstantsFollowSchema(t *testing.T) {
	t.Parallel()

	models := driveModelsDocument(t)
	models.Components.Schemas["DriveUploadValidationCookieName"].Value.Enum = []any{testUploadCookie}
	models.Components.Schemas["DriveUploadTokenCookieValue"].Value.Pattern = testUploadTokenPattern
	field := models.Components.Schemas["DriveUploadContentDisposition"].Value
	field.Pattern = `^synthetic; name="[^"]*"; filename="[^"]*"$`
	field.Extensions["x-protocol-template"] = `synthetic; name="%s"; filename="%s"`

	values, err := externalConstants(driveConstantsDocument(t), models)
	if err != nil {
		t.Fatal(err)
	}

	if values["DriveUploadValidationCookieNameValue"] != testUploadCookie ||
		values["DriveUploadTokenCookieValuePattern"] != testUploadTokenPattern ||
		values["DriveUploadContentDispositionTemplate"] != field.Extensions["x-protocol-template"] {
		t.Fatal("scalar constants did not follow their canonical declarations")
	}
}

func TestDriveScalarConstantsRejectInvalidFormats(t *testing.T) {
	t.Parallel()

	for _, value := range []any{1, "", `form-data; name="%d"; filename="%s"`, `other; name="%s"; filename="%s"`} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			t.Parallel()

			models := driveModelsDocument(t)
			models.Components.Schemas["DriveUploadContentDisposition"].Value.Extensions["x-protocol-template"] = value

			_, err := externalConstants(driveConstantsDocument(t), models)
			if !errors.Is(err, errConstant) {
				t.Fatal("invalid protocol template was emitted")
			}
		})
	}

	models := driveModelsDocument(t)
	models.Components.Schemas["DriveUploadTokenCookieValue"].Value.Pattern = "["

	_, err := externalConstants(driveConstantsDocument(t), models)
	if !errors.Is(err, errConstant) {
		t.Fatal("invalid cookie pattern was emitted")
	}
}
