package accountapi_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/internal/accountapi"
)

var (
	errPhotoRead  = errors.New("synthetic photo read failure")
	errPhotoClose = errors.New("synthetic photo close failure")
)

type photoBody struct {
	readErr  error
	closeErr error
	closed   int
}

func (body *photoBody) Read(_ []byte) (int, error) {
	if body.readErr != nil {
		return 0, body.readErr
	}

	return 0, io.EOF
}

func (body *photoBody) Close() error {
	body.closed++

	return body.closeErr
}

type photoRequester struct {
	body *photoBody
}

func (requester photoRequester) GetFamilyMemberPhoto(_ context.Context,
	_ *accountapi.GetFamilyMemberPhotoParams, _ ...accountapi.RequestEditorFn,
) (*http.Response, error) {
	response := new(http.Response)
	response.StatusCode = http.StatusOK
	response.Body = requester.body
	response.Header = make(http.Header)

	return response, nil
}

func TestMemberPhotoReadAndCloseFailure(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]*photoBody{
		"read":  {readErr: errPhotoRead, closeErr: nil, closed: 0},
		"close": {readErr: nil, closeErr: errPhotoClose, closed: 0},
		"both":  {readErr: errPhotoRead, closeErr: errPhotoClose, closed: 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			response, err := accountapi.ReadFamilyMemberPhoto(t.Context(), photoRequester{body: body},
				new(accountapi.GetFamilyMemberPhotoParams))
			if response != nil || err == nil || body.closed != 1 {
				t.Fatal("failed member photo returned data or lost body ownership")
			}

			for _, cause := range []error{body.readErr, body.closeErr} {
				if cause != nil && !errors.Is(err, cause) {
					t.Fatal("member photo lost a transport cause")
				}
			}
		})
	}
}
