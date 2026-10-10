//nolint:testpackage // GO-15: this unit verifies copying at the private selection boundary.
package icloud

import "testing"

func TestPhotoSelectionCopiesCallerState(t *testing.T) {
	t.Parallel()

	var auth AuthContext
	auth.AccountID = "synthetic-account"
	auth.ClientID = "synthetic-client"
	auth.PhotosServiceURL = "https://photos.example.invalid"
	auth.Headers = []Header{{Name: "X-Synthetic", Value: "before"}}
	const selectedZone = "synthetic-zone"

	var library PhotoLibrary

	library.ZoneName = selectedZone
	library.Shared = true
	library.ZoneType.Set("REGULAR_CUSTOM_ZONE")
	library.OwnerRecordName.Set("synthetic-owner")
	boundary, err := photosRequestContext(auth, &library)
	if err != nil {
		t.Fatal(err)
	}
	library.ZoneName = "changed-zone"
	library.ZoneType.Set("changed-type")
	library.OwnerRecordName.Set("changed-owner")
	auth.Headers[0].Value = "changed-header"
	zoneType, _ := boundary.PhotoZone.ZoneType.Get()
	owner, _ := boundary.PhotoZone.OwnerRecordName.Get()
	if boundary.PhotoZone.ZoneName != selectedZone || zoneType != "REGULAR_CUSTOM_ZONE" ||
		owner != "synthetic-owner" || boundary.Headers.Get("X-Synthetic") != "before" || !boundary.PhotoShared {
		t.Fatal("selected Photos request aliased caller-owned state")
	}
	primary, err := photosRequestContext(auth, nil)
	if err != nil {
		t.Fatal(err)
	}
	if primary.PhotoZone != nil || primary.PhotoShared {
		t.Fatal("SDK retained a previous caller's selection")
	}
}
