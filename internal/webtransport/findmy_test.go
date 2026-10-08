package webtransport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
)

type findMyRoundTrip func(*http.Request) (*http.Response, error)

func (call findMyRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return call(request)
}

func findMyTestAuth() webtransport.RequestContext {
	auth := new(webtransport.RequestContext)
	auth.Origin = "https://findmy.example.invalid"
	auth.Params.ClientId = "test-client"
	auth.Params.Dsid = "test-account"

	return *auth
}

func findMyTestResponse(body string) *http.Response {
	response := new(http.Response)
	response.StatusCode = http.StatusOK
	response.Header = make(http.Header)
	response.Header.Set(protocol.HTTPContentTypeName, protocol.FindMyMediaApplicationJson)
	response.Body = io.NopCloser(strings.NewReader(body))

	return response
}

func TestFindMyDiscoveryRejectsNonObjects(t *testing.T) {
	t.Parallel()

	for _, body := range []string{"null", "[]", "broken", `{"content":"invalid"}`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()

			client := webtransport.New(findMyRoundTrip(func(_ *http.Request) (*http.Response, error) {
				return findMyTestResponse(body), nil
			}))
			result, err := client.InitializeFindMy(t.Context(), findMyTestAuth(), false)

			var failure *webtransport.ResponseError

			if result != nil || !errors.As(err, &failure) || failure.Stage != webtransport.Decode ||
				failure.Status != http.StatusOK || string(failure.Body) != body {
				t.Fatalf("invalid discovery lost response evidence: %v", err)
			}
		})
	}
}

func TestFindMyTokenNullAndMissingAreDistinct(t *testing.T) {
	t.Parallel()

	auth := findMyTestAuth()
	auth.Headers = http.Header{protocol.CookieName: {"synthetic=original"}}
	original := auth.Headers.Clone()
	client := webtransport.New(findMyRoundTrip(func(request *http.Request) (*http.Response, error) {
		assertFindMyNullTokenRequest(t, request)

		return findMyTestResponse(`{"future": 9007199254740993}`), nil
	}))

	result, err := client.ObtainFindMyEraseToken(t.Context(), auth, nil)
	if err != nil || result == nil || result.Data.Tokens != nil ||
		string(result.Data.AdditionalProperties["future"]) != "9007199254740993" {
		t.Fatalf("missing token response cannot be inspected: %v", err)
	}

	if !reflect.DeepEqual(auth.Headers, original) {
		t.Fatal("token lookup changed caller-owned headers")
	}
}

func TestFindMyTokenMalformedResponse(t *testing.T) {
	t.Parallel()

	client := webtransport.New(findMyRoundTrip(func(_ *http.Request) (*http.Response, error) {
		return findMyTestResponse(`{"tokens": []}`), nil
	}))
	result, err := client.ObtainFindMyEraseToken(t.Context(), findMyTestAuth(), nil)

	var failure *webtransport.ResponseError

	if result != nil || !errors.As(err, &failure) || failure.Stage != webtransport.Decode {
		t.Fatalf("token response accepted malformed known fields: %v", err)
	}
}

func TestFindMyOriginRejectedBeforeNetwork(t *testing.T) {
	t.Parallel()

	auth := findMyTestAuth()
	auth.Origin = "https://findmy.example.invalid/extra"
	client := webtransport.New(findMyRoundTrip(func(_ *http.Request) (*http.Response, error) {
		t.Fatal("invalid service origin reached network")

		return nil, errFindMyTestOrigin
	}))
	result, err := client.InitializeFindMy(t.Context(), auth, false)

	var failure *webtransport.ResponseError

	if result != nil || !errors.As(err, &failure) || failure.Stage != webtransport.Configuration {
		t.Fatalf("invalid origin failure changed: %v", err)
	}
}

func TestFindMyCancellationPreservesCause(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := webtransport.New(findMyRoundTrip(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()

		return nil, request.Context().Err()
	}))
	result, err := client.PlayFindMySound(ctx, findMyTestAuth(), "test-device", "alert")

	var failure *webtransport.ResponseError

	if result != nil || !errors.As(err, &failure) || failure.Stage != webtransport.Transport ||
		!errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost its cause: %v", err)
	}
}

var errFindMyTestOrigin = errors.New("invalid test origin reached transport")

func TestFindMyRefreshPreservesOpaqueContextOrderAndOwnership(t *testing.T) {
	t.Parallel()

	server := findmy.FindMyRefreshContext(`{"z":1,"a":[null,{"nested":9007199254740993}],"theftLoss":{"discard":true}}`)
	original := append([]byte(nil), server...)
	client := webtransport.New(findMyRoundTrip(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}

		expected := `"serverContext": {"z": 1, "a": [null, {"nested": 9007199254740993}], "theftLoss": null}`
		if !strings.Contains(string(body), expected) {
			t.Fatal("refresh sorted opaque context or discarded unknown values")
		}

		return findMyTestResponse(`{"serverContext":{"z":1,"a":2},"content":[]}`), nil
	}))

	result, err := client.RefreshFindMy(t.Context(), findMyTestAuth(), server, true, false)
	if err != nil || result == nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual([]byte(server), original) {
		t.Fatal("refresh mutated caller-owned context")
	}

	for index := range result.Response.Body {
		result.Response.Body[index] = ' '
	}

	if string(result.Context) != `{"z":1,"a":2}` {
		t.Fatal("refresh context aliases caller-owned response bytes")
	}
}

func assertFindMyNullTokenRequest(t *testing.T, request *http.Request) {
	t.Helper()

	body, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != `{"dsWebAuthToken": null}` || request.URL.RawQuery != "" ||
		request.URL.Path != protocol.FindMyObtainEraseTokenPath {
		t.Fatal("token lookup invented authentication or account query parameters")
	}

	if request.Header.Get(protocol.AcceptName) != "*/*" ||
		request.Header.Get(protocol.HTTPContentTypeName) != protocol.FindMyMediaApplicationJson {
		t.Fatal("token lookup lost default headers")
	}
}
