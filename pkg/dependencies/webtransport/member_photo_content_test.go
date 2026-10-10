package webtransport_test

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
)

var errMemberPhotoRead = errors.New("synthetic member photo read failure")

type memberPhotoFailureBody struct {
	readErr  error
	closeErr error
	closed   int
}

func (body *memberPhotoFailureBody) Read(_ []byte) (int, error) {
	if body.readErr != nil {
		return 0, body.readErr
	}

	return 0, io.EOF
}

func (body *memberPhotoFailureBody) Close() error {
	body.closed++

	return body.closeErr
}

// These synthetic controls exercise the active account transport rather than
// the removed internal generated sender's unused binary response adapter.
func TestMemberPhotoResponseReadAndCloseFailures(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]*memberPhotoFailureBody{
		"read":  {readErr: errMemberPhotoRead, closeErr: nil, closed: 0},
		"close": {readErr: nil, closeErr: errContentClose, closed: 0},
		"both":  {readErr: errMemberPhotoRead, closeErr: errContentClose, closed: 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			response := new(http.Response)
			response.StatusCode = http.StatusOK
			response.Header = make(http.Header)
			response.Body = body

			client := webtransport.New(findMyRoundTrip(func(_ *http.Request) (*http.Response, error) {
				return response, nil
			}))

			auth := findMyTestAuth()
			auth.Headers = make(http.Header)

			result, err := client.GetMemberPhoto(t.Context(), auth, "synthetic-member")
			if result != nil || err == nil || body.closed != 1 {
				t.Fatalf("member photo failure lost body ownership: result=%+v error=%v closes=%d", result, err, body.closed)
			}

			for _, cause := range []error{body.readErr, body.closeErr} {
				if cause != nil && !errors.Is(err, cause) {
					t.Fatal("member photo transport lost its failure cause")
				}
			}
		})
	}
}
