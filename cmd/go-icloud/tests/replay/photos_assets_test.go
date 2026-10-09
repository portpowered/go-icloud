package replay_test

import (
	"encoding/json"
	"testing"
	"time"
)

func checkPhotoAssetsCLI(t *testing.T, row, actual map[string]json.RawMessage) {
	t.Helper()

	if len(actual) != 1 {
		t.Fatal("photo assets CLI exposed unexpected fields")
	}

	var expected []map[string]json.RawMessage

	decode(t, row["result"], &expected)

	for _, photo := range expected {
		for old, name := range map[string]string{
			"master_id": "masterID", "item_type": "itemType",
			"is_live_photo": "isLivePhoto", "asset": "assetMetadata",
		} {
			photo[name] = photo[old]
			delete(photo, old)
		}

		for _, field := range []string{"created", "added"} {
			var instant time.Time

			decode(t, photo[field], &instant)

			value, err := json.Marshal(instant.UTC())
			if err != nil {
				t.Fatal(err)
			}

			photo[field] = value
		}
	}

	checkReminderCLIValue(t, actual["photos"], expected)
}
