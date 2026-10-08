package webtransport

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/drive"
)

// DriveUploadInput identifies a file whose cursor and lifetime belong to the caller.
type DriveUploadInput struct {
	ParentID         string
	Filename         string
	Content          io.ReadSeeker
	Zone             string
	ModificationTime *time.Time
	CreationTime     *time.Time
	Clock            func() time.Time
}

// DriveUploadResponse preserves every completed response and registration acknowledgement.
type DriveUploadResponse struct {
	Token        string
	Destination  drive.DriveUploadDestination
	Receipt      drive.DriveUploadReceipt
	Registration drive.DriveUpdatedDocuments
	Preparation  *BytesResponse
	Transfer     *BytesResponse
	Registered   *BytesResponse
}

// UploadDriveFile prepares, transfers and registers without retrying an uncertain write.
func (client *Client) UploadDriveFile(ctx context.Context, auth RequestContext,
	input DriveUploadInput,
) (*DriveUploadResponse, error) {
	preparation, destination, token, err := client.prepareDriveUpload(ctx, auth, input)
	if err != nil {
		return nil, withUploadPrior(err, token)
	}

	transfer, receipt, err := client.transferDriveUpload(ctx, auth, input, destination.Url)
	if err != nil {
		return nil, withUploadPrior(err, token, preparation)
	}

	registered, data, err := client.registerDriveUpload(ctx, auth, input, destination.DocumentId,
		token, receipt.SingleFile)
	if err != nil {
		return nil, withUploadPrior(err, token, preparation, transfer)
	}

	return &DriveUploadResponse{Token: token, Destination: destination, Receipt: receipt, Registration: data,
		Preparation: preparation, Transfer: transfer, Registered: registered}, nil
}

func withUploadPrior(err error, token string, responses ...*BytesResponse) error {
	var responseError *ResponseError

	if errors.As(err, &responseError) {
		responseError.Prior = responses
		responseError.UploadToken = token
	}

	return err
}
