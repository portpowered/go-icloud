package webtransport_test

import (
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

func TestPhotosGeneratedAssetQueryPreservesSourceBytes(t *testing.T) {
	t.Parallel()

	body, err := webtransport.ExportPhotosAssetBodyLimit(cloudkit.CPLAssetAndMasterByAddedDate, cloudkit.ASCENDING, -1,
		[]webtransport.PhotosAssetSelector{{Field: cloudkit.PhotoAssetQueryFieldRecordName, Value: `synthetic<&>"`}}, 2)
	if err != nil {
		t.Fatal(err)
	}

	const expected = `{"query": {"recordType": "CPLAssetAndMasterByAddedDate", "filterBy": [` +
		`{"comparator": "EQUALS", "fieldName": "direction", "fieldValue": {"type": "STRING", "value": "ASCENDING"}}, ` +
		`{"comparator": "EQUALS", "fieldName": "startRank", "fieldValue": {"type": "INT64", "value": 0}}, ` +
		`{"comparator": "EQUALS", "fieldName": "recordName", "fieldValue": {"type": "STRING", ` +
		`"value": "synthetic<&>\""}}]}, ` +
		`"zoneID": {"zoneName": "PrimarySync", "zoneType": "REGULAR_CUSTOM_ZONE"}, "resultsLimit": 2}`

	if body != expected {
		t.Fatalf("asset query bytes = %s, want %s", body, expected)
	}
}

func TestPhotosGeneratedAlbumQueryPreservesSourceBytes(t *testing.T) {
	t.Parallel()

	parent, continuation := "parent<&>", "next<&>"
	zone := new(cloudkit.CKZoneIDReq)
	zone.ZoneName = "synthetic-é"
	zone.ZoneType.Set("REGULAR_CUSTOM_ZONE")
	zone.OwnerRecordName.Set("synthetic-owner")

	body, err := webtransport.ExportPhotosAlbumBody(&parent, &continuation, zone)
	if err != nil {
		t.Fatal(err)
	}

	const expected = `{"query": {"recordType": "CPLAlbumByPositionLive", "filterBy": [` +
		`{"comparator": "EQUALS", "fieldName": "parentId", "fieldValue": {"type": "STRING", "value": "parent<&>"}}]}, ` +
		`"zoneID": {"zoneName": "synthetic-\u00e9", "zoneType": "REGULAR_CUSTOM_ZONE", ` +
		`"ownerRecordName": "synthetic-owner"}, "continuationMarker": "next<&>"}`

	if body != expected {
		t.Fatalf("album query bytes = %s, want %s", body, expected)
	}
}
