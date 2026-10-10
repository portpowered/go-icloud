package photomaterialize_test

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/internal/photomaterialize"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"howett.net/plist"
)

const fieldValueKey = "value"

func TestSourceMaterializationOracle(t *testing.T) {
	t.Parallel()

	oracle := readOracle(t)
	stamp := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	exif := photomaterialize.UpdateEXIF([]byte{0xff, 0xd8, 0xff, 0xd9}, stamp)
	if hex.EncodeToString(exif) != oracle.ExifHex {
		t.Fatalf("EXIF does not match complete pinned Source output: %x", exif)
	}

	if !bytes.Equal(photomaterialize.UpdateEXIF(exif, stamp.Add(time.Hour)), exif) {
		t.Fatal("existing EXIF timestamp changed")
	}

	metadata := oracleMetadata(t)
	if actual := string(photomaterialize.RenderXMP(metadata)); actual != oracle.Xmp {
		t.Fatalf("XMP does not match complete pinned Source output:\n%s", actual)
	}

	if !photomaterialize.OwnedXMP([]byte(oracle.Xmp)) {
		t.Fatal("Source generated sidecar was not recognized")
	}
}

func oracleMetadata(t *testing.T) photomaterialize.Metadata {
	t.Helper()

	metadata := new(photomaterialize.Metadata)

	data := []byte(`{"toolkit":"pyicloud photos-cloudkit","title":"A <photo> & title",` +
		`"description":"one\ntwo","orientation":0,"make":"Screenshot","digitalSourceType":"screenCapture",` +
		`"keywords":["hello",""],"gpsAltitude":0.0,"gpsLatitude":51.5,"gpsLongitude":-0.5,"gpsSpeed":3.0,` +
		`"gpsTimestamp":"2024-01-02T03:04:05Z","createDate":"2024-01-02T03:04:05-08:00","rating":-1}`)

	err := json.Unmarshal(data, metadata)
	if err != nil {
		t.Fatal(err)
	}

	return *metadata
}

func readOracle(t *testing.T) photomaterialize.MaterializationOracle {
	t.Helper()

	data, err := os.ReadFile("../../tests/replay/fixtures/synthetic/local/photos-materialization.json")
	if err != nil {
		t.Fatal(err)
	}

	oracle := new(photomaterialize.MaterializationOracle)

	err = json.Unmarshal(data, oracle)
	if err != nil {
		t.Fatal(err)
	}

	return *oracle
}

func TestPreserveForeignAndMalformedXMP(t *testing.T) {
	t.Parallel()

	oracle := readOracle(t).Xmp
	for _, document := range []string{
		"", "garbage", "<unclosed>", strings.ReplaceAll(oracle, "pyicloud photos-cloudkit", "Lightroom"),
		oracle + "<second />", oracle + "junk", oracle[:len(oracle)-5],
	} {
		if photomaterialize.OwnedXMP([]byte(document)) {
			t.Fatalf("unsafe sidecar accepted: %q", document)
		}
	}

	if !photomaterialize.OwnedXMP([]byte(strings.ReplaceAll(oracle,
		"pyicloud photos-cloudkit", "pyicloud photos-cloudkit 2.0"))) {
		t.Fatal("Source toolkit prefix was rejected")
	}
}

func TestRAWAlignment(t *testing.T) {
	t.Parallel()

	extensions := []string{"ARW", "cr2", "cr3", "crw", "dng", "nef", "nrf", "nrw", "orf", "pef", "raf", "rw2"}
	for _, extension := range extensions {
		if !photomaterialize.ResourceIsRAW("picture."+extension, "") {
			t.Fatalf("Source RAW extension rejected: %s", extension)
		}
	}

	if !photomaterialize.ResourceIsRAW("unknown", "com.apple.RAW-image") ||
		photomaterialize.ResourceIsRAW("photo.jpeg", "public.image") {
		t.Fatal("RAW content type recognition differs")
	}

	if !photomaterialize.ShouldSwapRAW("photo.jpg", "", "photo.nef", "", "original") ||
		!photomaterialize.ShouldSwapRAW("photo.nef", "", "photo.jpg", "", "alternative") {
		t.Fatal("RAW alignment did not swap conflicting preference")
	}

	for _, policy := range []string{"as-is", "unknown", "alternative"} {
		if photomaterialize.ShouldSwapRAW("photo.jpg", "", "photo.nef", "", policy) {
			t.Fatalf("unexpected alignment for %s", policy)
		}
	}
}

func TestMalformedEXIFAndPreservation(t *testing.T) {
	t.Parallel()

	stamp := time.Unix(0, 0)
	for _, input := range [][]byte{nil, {}, {0xff}, []byte("not a JPEG")} {
		if !bytes.Equal(photomaterialize.UpdateEXIF(input, stamp), input) {
			t.Fatal("invalid JPEG was changed")
		}
	}

	for _, input := range [][]byte{
		{0xff, 0xd8}, {0xff, 0xd8, 1, 2}, {0xff, 0xd8, 0xff, 0xe1, 0xff, 0xff},
		{0xff, 0xd8, 0xff, 0xe1, 0, 1}, {0xff, 0xd8, 0xff, 0xda, 0, 2},
	} {
		actual := photomaterialize.UpdateEXIF(input, stamp)
		if !bytes.Equal(actual[len(actual)-len(input)+2:], input[2:]) {
			t.Fatal("original JPEG bytes were not preserved")
		}
	}
}

func TestExtractEmbeddedMetadata(t *testing.T) {
	t.Parallel()

	for _, format := range []int{plist.XMLFormat, plist.BinaryFormat} {
		metadata := extractEmbedded(t, format)
		assertEmbeddedGPS(t, metadata)
		assertEmbeddedText(t, metadata)
	}
}

func assertEmbeddedGPS(t *testing.T, metadata *photomaterialize.Metadata) {
	t.Helper()

	if metadata.GpsLatitude == nil || *metadata.GpsLatitude != 51.5 || metadata.GpsAltitude != nil ||
		metadata.GpsTimestamp == nil || metadata.Rating == nil || *metadata.Rating != -1 {
		t.Fatalf("embedded GPS mismatch: %+v", metadata)
	}
}

func assertEmbeddedText(t *testing.T, metadata *photomaterialize.Metadata) {
	t.Helper()

	if metadata.Title == nil || *metadata.Title != "A photo" || metadata.Orientation == nil ||
		*metadata.Orientation != 6 || metadata.Keywords == nil || strings.Join(*metadata.Keywords, ",") != "one,two" {
		t.Fatalf("embedded text mismatch: %+v", metadata)
	}

	assertEmbeddedDates(t, metadata)
}

func assertEmbeddedDates(t *testing.T, metadata *photomaterialize.Metadata) {
	t.Helper()

	if metadata.Make == nil || *metadata.Make != "Screenshot" || metadata.CreateDate == nil ||
		metadata.CreateDate.Format("-0700") != "-0800" {
		t.Fatalf("metadata projection differs: %+v", metadata)
	}
}

func extractEmbedded(t *testing.T, format int) *photomaterialize.Metadata {
	t.Helper()

	keywords, err := plist.Marshal([]string{"one", "two"}, format)
	if err != nil {
		t.Fatal(err)
	}

	location, err := plist.Marshal(map[string]any{
		"lat": 51.5, "lon": -0.5, "alt": "invalid", "speed": 3,
		"timestamp": time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}, format)
	if err != nil {
		t.Fatal(err)
	}

	var adjustment bytes.Buffer

	writer, err := flate.NewWriter(&adjustment, flate.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}

	_, err = writer.Write([]byte(`{"metadata":{"orientation":6}}`))
	if err != nil {
		t.Fatal(err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	fields := map[string]any{
		"captionEnc":              map[string]any{fieldValueKey: base64.StdEncoding.EncodeToString([]byte("A photo"))},
		"keywordsEnc":             map[string]any{fieldValueKey: base64.StdEncoding.EncodeToString(keywords)},
		"locationEnc":             map[string]any{fieldValueKey: base64.StdEncoding.EncodeToString(location)},
		"adjustmentSimpleDataEnc": map[string]any{fieldValueKey: base64.StdEncoding.EncodeToString(adjustment.Bytes())},
		"assetDate":               map[string]any{fieldValueKey: 1704186245000},
		"timeZoneOffset":          map[string]any{fieldValueKey: -28800},
		"isHidden":                map[string]any{fieldValueKey: 1}, "isFavorite": map[string]any{fieldValueKey: 1},
		"assetSubtypeV2": map[string]any{fieldValueKey: 3},
	}

	data, err := json.Marshal(map[string]any{"recordName": "synthetic", "recordType": "CPLAsset", "fields": fields})
	if err != nil {
		t.Fatal(err)
	}

	record := new(cloudkit.CKRecord)

	err = json.Unmarshal(data, record)
	if err != nil {
		t.Fatal(err)
	}

	return photomaterialize.ExtractMetadata(*record)
}
