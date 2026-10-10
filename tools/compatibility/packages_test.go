package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPublicPackageInventoryRetainsRemovedAndAddedPackages(t *testing.T) {
	t.Parallel()
	current, baseline := t.TempDir(), t.TempDir()
	writePublicPackageFixture(t, current, "pkg/icloud/client.go")
	writePublicPackageFixture(t, current, "pkg/photosync/sync.go")
	writePublicPackageFixture(t, baseline, "pkg/icloud/client.go")
	writePublicPackageFixture(t, baseline, "pkg/dependencymodels/legacy/model.go")
	writePublicPackageFixture(t, current, "pkg/testonly/fixture_test.go")
	packages, err := expandPublicPackages(current, baseline, []string{defaultPublicPackages})
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"pkg/dependencymodels/legacy", "pkg/icloud", "pkg/photosync"}
	if !reflect.DeepEqual(packages, expected) {
		t.Fatalf("public package denominator: want %v, got %v", expected, packages)
	}
	for path, expectedStatus := range map[string]publicPackageStatus{
		"pkg/icloud": publicPackagePresent, "pkg/photosync": publicPackageAdded,
		"pkg/dependencymodels/legacy": publicPackageRemoved,
	} {
		status, err := publicPackagePresence(current, baseline, path)
		if err != nil || status != expectedStatus {
			t.Fatalf("%s presence=%v, error=%v", path, status, err)
		}
	}
}

func TestPublicPackageInventoryFailsClosedWithoutProductionPackages(t *testing.T) {
	t.Parallel()
	current, baseline := t.TempDir(), t.TempDir()
	writePublicPackageFixture(t, current, "pkg/testonly/fixture_test.go")
	if _, err := expandPublicPackages(current, baseline, []string{defaultPublicPackages}); err == nil {
		t.Fatal("empty public inventory was accepted")
	}
	if _, err := publicPackagePresence(current, baseline, "pkg/missing"); err == nil {
		t.Fatal("missing explicit public package was accepted")
	}
}

func writePublicPackageFixture(t *testing.T, root, path string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
