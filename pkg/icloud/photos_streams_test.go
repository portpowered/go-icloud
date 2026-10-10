package icloud_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const canceledSharedPhotoID = "synthetic-shared-photo"
const sharedPhotoCountOperation = "count"

func TestSharedPhotosCanceledBeforeNetwork(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{"albums", sharedPhotoCountOperation, "list", "get", "download"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()

			client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(
				func(_ *http.Request) (*http.Response, error) {
					t.Fatal("canceled shared photos operation reached transport")

					return nil, errSyntheticPeer
				})))
			if err != nil {
				t.Fatal(err)
			}

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err = callCanceledSharedPhotos(ctx, client, operation)

			var failure *icloud.ClientError

			if !errors.As(err, &failure) || failure.Kind() != icloud.Canceled || !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation cause or classification changed", err)
			}
		})
	}
}

//nolint:wrapcheck // LIB-05: preserve the SDK error for exact cancellation assertions.
func callCanceledSharedPhotos(ctx context.Context, client *icloud.SDK, operation string) error {
	auth := deviceRequest().Auth

	switch operation {
	case "albums":
		_, err := client.ListSharedPhotoAlbums(ctx, icloud.ListSharedPhotoAlbumsRequest{Auth: auth})

		return err
	case sharedPhotoCountOperation:
		_, err := client.CountSharedPhotos(ctx, icloud.CountSharedPhotosRequest{Auth: auth, Album: canceledSharedPhotoID})

		return err
	case "list":
		_, err := client.ListSharedPhotos(ctx, icloud.ListSharedPhotosRequest{Auth: auth, Album: canceledSharedPhotoID})

		return err
	case "get":
		_, err := client.GetSharedPhoto(ctx, icloud.GetSharedPhotoRequest{
			Auth: auth, Album: canceledSharedPhotoID, PhotoID: canceledSharedPhotoID})

		return err
	default:
		_, err := client.DownloadSharedPhoto(ctx, icloud.DownloadSharedPhotoRequest{
			Auth: auth, Album: canceledSharedPhotoID, PhotoID: canceledSharedPhotoID, Version: nil})

		return err
	}
}
