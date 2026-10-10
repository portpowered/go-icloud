package accountapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// MemberPhotoResponse retains exact member-photo bytes and response metadata.
// The service may label binary success with a JSON content type; do not decode it.
type MemberPhotoResponse struct {
	Body    []byte
	Status  int
	Headers http.Header
}

// MemberPhotoRequester is the generated request seam used by the binary adapter.
type MemberPhotoRequester interface {
	GetFamilyMemberPhoto(ctx context.Context, params *GetFamilyMemberPhotoParams,
		edits ...RequestEditorFn) (*http.Response, error)
}

// ReadFamilyMemberPhoto owns and closes the generated client's response body.
// Failures retain their status and exact bytes for the SDK's error adapter.
func ReadFamilyMemberPhoto(ctx context.Context, client MemberPhotoRequester,
	params *GetFamilyMemberPhotoParams,
) (*MemberPhotoResponse, error) {
	response, err := client.GetFamilyMemberPhoto(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("request family member photo: %w", err)
	}

	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()

	err = errors.Join(readErr, closeErr)
	if err != nil {
		return nil, fmt.Errorf("read family member photo: %w", err)
	}

	return &MemberPhotoResponse{
		Body: body, Status: response.StatusCode, Headers: response.Header.Clone(),
	}, nil
}
