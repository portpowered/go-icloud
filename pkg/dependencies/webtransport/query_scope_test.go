package webtransport_test

import (
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
)

// Synthetic controls exercise the shared transport context independently of SDK projections.
func TestDriveTokenQueryScope(t *testing.T) {
	t.Parallel()

	for _, drive := range []bool{false, true} {
		t.Run(map[bool]string{false: "account", true: "drive"}[drive], func(t *testing.T) {
			t.Parallel()

			auth := findMyTestAuth()
			auth.DriveToken = "caller token&value"
			calls := 0
			client := webtransport.New(findMyRoundTrip(func(request *http.Request) (*http.Response, error) {
				calls++
				got := request.URL.Query().Get(protocol.DriveTokenName)
				if drive && got != auth.DriveToken || !drive && got != "" {
					t.Fatalf("Drive token escaped operation scope: drive=%v query=%q", drive, request.URL.RawQuery)
				}

				if request.URL.Query().Get(protocol.ClientIDName) != auth.Params.ClientId {
					t.Fatal("account query was lost")
				}

				if drive {
					return findMyTestResponse(`{"items":[]}`), nil
				}

				return findMyTestResponse(`{"devices":[]}`), nil
			}))

			var err error
			if drive {
				_, err = client.ListDriveLibraries(t.Context(), auth)
			} else {
				_, err = client.GetDevices(t.Context(), auth)
			}

			if err != nil || calls != 1 {
				t.Fatalf("scoped request failed: calls=%d error=%v", calls, err)
			}
		})
	}
}
