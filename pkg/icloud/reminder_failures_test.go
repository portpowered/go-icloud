package icloud_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

func TestReminderWritesPreserveHTTPAndInvalidResponseFailures(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{reminderCreateTestOperation,
		reminderUpdateTestOperation, reminderDeleteTestOperation, reminderHashtagCreateTestOperation,
		reminderHashtagUpdateTestOperation, reminderHashtagDeleteTestOperation} {
		for name, row := range map[string]struct {
			body   string
			status int
			kind   icloud.ErrorKind
		}{
			"http-unavailable": {body: `{"reason":"synthetic provider failure"}`,
				status: http.StatusServiceUnavailable, kind: icloud.Unavailable},
			testInvalidJSON:   {body: `not JSON`, status: http.StatusOK, kind: icloud.InvalidResponse},
			"null-records":    {body: `{"records":null}`, status: http.StatusOK, kind: icloud.InvalidResponse},
			"invalid-records": {body: `{"records":[false]}`, status: http.StatusOK, kind: icloud.InvalidResponse},
		} {
			t.Run(operation+"/"+name, func(t *testing.T) {
				t.Parallel()
				client := sdkForResponse(t, row.status, row.body)
				err := callReminderMutation(t.Context(), client, operation)

				var failure *icloud.ClientError

				if !errors.As(err, &failure) || failure.Kind() != row.kind ||
					failure.StatusCode() != row.status || string(failure.ResponseBody()) != row.body {
					t.Fatal("mutation lost HTTP or invalid-response evidence", err)
				}

				if len(failure.ResponseHeaders()) == 0 || failure.CookieScopeURL() == "" ||
					len(failure.PriorResponses()) != 0 {
					t.Fatal("mutation lost response headers or cookie scope")
				}
			})
		}
	}
}
