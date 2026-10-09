package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

type visitorProbe struct {
	icloud.Client
	want   icloud.AuthContext
	output *bytes.Buffer
	cancel context.CancelFunc
	visits int
}

func (probe *visitorProbe) VisitPhotoAssets(_ context.Context, request icloud.ListPhotoAssetsRequest,
	visitor icloud.PhotoVisitor,
) (*icloud.ListPhotoAssetsResult, error) {
	if err := probe.deliver(request.Auth, visitor); err != nil {
		return nil, err
	}
	var result icloud.ListPhotoAssetsResult
	result.Photos = []icloud.Photo{}
	result.Responses = []icloud.ResponseMetadata{}
	return &result, nil
}

func (probe *visitorProbe) VisitRecentlyAddedPhotos(_ context.Context, request icloud.ListRecentlyAddedPhotosRequest,
	visitor icloud.PhotoVisitor,
) (*icloud.ListRecentlyAddedPhotosResult, error) {
	if err := probe.deliver(request.Auth, visitor); err != nil {
		return nil, err
	}
	var result icloud.ListRecentlyAddedPhotosResult
	result.Photos = []icloud.Photo{}
	result.Responses = []icloud.ResponseMetadata{}
	return &result, nil
}

func (probe *visitorProbe) deliver(auth icloud.AuthContext, visitor icloud.PhotoVisitor) error {
	if !reflect.DeepEqual(auth, probe.want) {
		return errors.New("foreign authentication reached SDK")
	}
	if probe.cancel != nil {
		probe.cancel()
	}
	for _, id := range []string{"first", "second"} {
		var event icloud.PhotoVisitEvent
		event.Photo.ID = id
		event.Photo.AssetMetadata = json.RawMessage(`{"private":"synthetic-stream-secret"}`)
		keepGoing, err := visitor(event)
		if err != nil {
			return err
		}
		if !keepGoing {
			return errors.New("visitor unexpectedly stopped")
		}
		probe.visits++
		if probe.output != nil && !strings.Contains(probe.output.String(), id) {
			return errors.New("visitor did not emit synchronously")
		}
	}
	return nil
}

func TestPhotoVisitorsStreamBeforeNextCallback(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"photo-assets-visit", "photos-recently-added-visit"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			var probe visitorProbe
			probe.want.AccountID = "stored"
			probe.output = &output
			input, saved := visitorPaths(t)
			if err := runPhotoVisit(t.Context(), &probe, probe.want, operation, input, saved, &output); err != nil {
				t.Fatal(err)
			}
			if probe.visits != 2 || strings.Contains(output.String(), "synthetic-stream-secret") {
				t.Fatal("stream count or privacy differs")
			}
			decoder := json.NewDecoder(&output)
			for range 3 {
				var event map[string]json.RawMessage
				if err := decoder.Decode(&event); err != nil {
					t.Fatal(err)
				}
			}
			var extra any
			if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
				t.Fatal("unexpected stream messages")
			}
		})
	}
}

func TestPhotoVisitorCanceledBeforeEmissionPreservesReceipt(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var probe visitorProbe
	probe.cancel = cancel
	input, saved := visitorPaths(t)
	previous := []byte("previous receipt")
	if err := os.WriteFile(saved, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err := runPhotoVisit(ctx, &probe, probe.want, "photo-assets-visit", input, saved, &output)
	if !errors.Is(err, context.Canceled) || output.Len() != 0 || probe.visits != 0 {
		t.Fatal("cancellation did not stop emission", err)
	}
	actual, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, previous) {
		t.Fatal("cancellation overwrote existing receipt")
	}
}

func visitorPaths(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	input, saved := filepath.Join(directory, "request.json"), filepath.Join(directory, "result.json")
	if err := os.WriteFile(input, []byte(`{"auth":{"accountID":"foreign"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return input, saved
}
