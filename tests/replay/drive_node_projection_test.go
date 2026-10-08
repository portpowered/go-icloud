package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const referenceDriveDateLayout = "2006-01-02T15:04:05"

const (
	nodeRootLabel    = "root"
	nodeTrashLabel   = "trash"
	nodeRefreshRoot  = "refresh_root"
	nodeRefreshTrash = "refresh_trash"
	nodeBodyKey      = "body"
	nodeStatusKey    = "status"
)

func portableDriveDate(value *time.Time) any {
	if value == nil {
		return nil
	}

	return value.Format(referenceDriveDateLayout)
}

func portableDriveSnapshot(t *testing.T, entry *icloud.DriveEntry) any {
	t.Helper()

	if entry == nil {
		return nil
	}

	value, err := entry.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	return map[string]any{"data": value.Data, "name": value.Name, "type": value.Type, "size": value.Size,
		"date_changed": portableDriveDate(value.DateChanged), "date_modified": portableDriveDate(value.DateModified),
		"date_last_open": portableDriveDate(value.DateLastOpen)}
}

func portableDriveChildren(t *testing.T, entries []*icloud.DriveEntry) []any {
	t.Helper()

	values := make([]any, 0, len(entries))
	for _, entry := range entries {
		values = append(values, portableDriveSnapshot(t, entry))
	}

	return values
}

func portableDriveDownloaded(t *testing.T, value *icloud.DriveEntryDownloadResult) any {
	t.Helper()

	var (
		status  any
		headers any = []any{}
	)

	if value.Metadata != nil {
		status = value.Metadata.StatusCode

		pairs := make([][2]string, 0, len(value.Metadata.Headers))
		for _, header := range value.Metadata.Headers {
			pairs = append(pairs, [2]string{header.Name, header.Value})
		}

		headers = pairs
	}

	return map[string]any{nodeStatusKey: status, "headers": headers,
		nodeBodyKey: base64.StdEncoding.EncodeToString(value.Content)}
}

func portableDriveChanged(t *testing.T, value *icloud.DriveItemChangeResult) any {
	t.Helper()

	fields := make(map[string]any, len(value.AdditionalMetadata)+1)
	for name, raw := range value.AdditionalMetadata {
		fields[name] = raw
	}

	if value.Items != nil {
		fields["items"] = value.Items
	}

	return fields
}

func driveNodeKeyword(t *testing.T, call driveNodeCall, key string, output any) {
	t.Helper()

	value, exists := call.Keywords[key]
	if !exists {
		return
	}

	err := json.Unmarshal(value, output)
	if err != nil {
		t.Fatal(err)
	}
}
