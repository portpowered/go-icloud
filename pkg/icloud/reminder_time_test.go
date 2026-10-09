package icloud_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const reminderDeletionEntropyBytes = 32

func TestReminderWriteTimestampReferenceTruncation(t *testing.T) {
	t.Parallel()

	for _, row := range []struct {
		instant time.Time
		millis  int64
	}{
		{instant: time.Unix(0, 500000), millis: 0},
		{instant: time.Unix(-1, 999500000), millis: 0},
		{instant: time.Unix(1700000000, 123456789), millis: 1700000000123},
		{instant: time.Unix(-1, 998500000), millis: -1},
	} {
		t.Run(row.instant.String(), func(t *testing.T) {
			t.Parallel()
			checkReminderWriteTimestamp(t, row.instant, row.millis)
		})
	}
}

func checkReminderWriteTimestamp(t *testing.T, instant time.Time, millis int64) {
	t.Helper()

	client, err := icloud.New(icloud.WithClock(func() time.Time { return instant }),
		icloud.WithRandomSource(bytes.NewReader(make([]byte, reminderDeletionEntropyBytes))),
		icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
			var payload cloudkit.ReminderDeletionRequest

			decodeErr := json.NewDecoder(request.Body).Decode(&payload)
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}

			if len(payload.Operations) != 1 ||
				payload.Operations[0].Record.Fields.LastModifiedDate.Value != millis {
				t.Fatal("wire timestamp differs from reference int(seconds * 1000)")
			}

			response := new(http.Response)
			response.StatusCode = http.StatusOK
			response.Body = io.NopCloser(strings.NewReader(`{}`))
			response.Header = make(http.Header)

			return response, nil
		})))
	if err != nil {
		t.Fatal(err)
	}

	auth := deviceRequest().Auth
	auth.RemindersServiceURL = reminderTestOrigin

	result, err := client.DeleteReminder(t.Context(), icloud.DeleteReminderRequest{
		Auth: auth, ReminderID: reminderTestRecordName, RecordChangeTag: nil})
	if err != nil {
		t.Fatal(err)
	}

	if result.Modified.UnixMilli() != millis {
		t.Fatal("returned timestamp differs from acknowledged wire timestamp")
	}
}
