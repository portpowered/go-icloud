package icloud_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
	"testing"
)

type photoMutationCancellationCall func(context.Context, *icloud.SDK) error

func TestPhotoMutationCanceledBeforeNetworkOrEntropy(t *testing.T) {
	t.Parallel()
	calls := map[string]photoMutationCancellationCall{
		"CreatePhotoAlbum": func(ctx context.Context, sdk *icloud.SDK) error {
			_, err := sdk.CreatePhotoAlbum(ctx, icloud.CreatePhotoAlbumRequest{Auth: icloud.AuthContext{}, Name: "album", Folder: false, Library: nil})
			return err
		},
		"RenamePhotoAlbum": func(ctx context.Context, sdk *icloud.SDK) error {
			_, err := sdk.RenamePhotoAlbum(ctx, icloud.RenamePhotoAlbumRequest{Auth: icloud.AuthContext{}, AlbumID: "album", Name: "name", Library: nil})
			return err
		},
		"DeletePhotoAlbum": func(ctx context.Context, sdk *icloud.SDK) error {
			_, err := sdk.DeletePhotoAlbum(ctx, icloud.DeletePhotoAlbumRequest{Auth: icloud.AuthContext{}, AlbumID: "album", Library: nil})
			return err
		},
		"AddPhotoToAlbum": func(ctx context.Context, sdk *icloud.SDK) error {
			_, err := sdk.AddPhotoToAlbum(ctx, icloud.AddPhotoToAlbumRequest{Auth: icloud.AuthContext{}, AlbumID: "album", PhotoID: "photo", Library: nil})
			return err
		},
		"SetPhotoFavorite": func(ctx context.Context, sdk *icloud.SDK) error {
			_, err := sdk.SetPhotoFavorite(ctx, icloud.SetPhotoFavoriteRequest{Auth: icloud.AuthContext{}, PhotoID: "photo", Favorite: true, Album: nil, Library: nil})
			return err
		},
		"DeletePhoto": func(ctx context.Context, sdk *icloud.SDK) error {
			_, err := sdk.DeletePhoto(ctx, icloud.DeletePhotoRequest{Auth: icloud.AuthContext{}, PhotoID: "photo", Album: nil, Library: nil})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) { t.Parallel(); checkCanceledPhotoMutation(t, call) })
	}
}
func checkCanceledPhotoMutation(t *testing.T, call photoMutationCancellationCall) {
	t.Helper()
	transport, err := replay.NewHTTPTransport(nil)
	if err != nil {
		t.Fatal(err)
	}
	entropy := bytes.NewReader([]byte{1})
	sdk, err := icloud.New(icloud.WithHTTPTransport(transport), icloud.WithRandomSource(entropy))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err = call(ctx, sdk)
	var failure *icloud.ClientError
	if !errors.As(err, &failure) || failure.Kind() != icloud.Canceled || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation class lost", err)
	}
	if entropy.Len() != 1 {
		t.Fatal("canceled mutation consumed entropy")
	}
	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
