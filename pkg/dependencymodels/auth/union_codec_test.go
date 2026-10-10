package auth_test

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport/authapi"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// SCHEMA-11: synthetic controls exercise both primitive object schema members.
func TestLoginUnionPreservesConstructorBytes(t *testing.T) {
	t.Parallel()

	token := auth.AuthTokenLoginRequest{AccountCountryCode: nil, DsWebAuthToken: "token", ExtendedLogin: true, TrustToken: "trust"}
	credentials := auth.AuthCredentialsLoginRequest{AppName: nil, AppleId: "account", Password: "password"}

	for _, test := range []struct {
		name  string
		model any
		from  func(*auth.AuthLoginRequest) error
		as    func(auth.AuthLoginRequest) (any, error)
	}{
		{"token", token, func(body *auth.AuthLoginRequest) error {
			return body.FromAuthTokenLoginRequest(token)
		}, func(body auth.AuthLoginRequest) (any, error) {
			return body.AsAuthTokenLoginRequest()
		}},
		{"credentials", credentials, func(body *auth.AuthLoginRequest) error {
			return body.FromAuthCredentialsLoginRequest(credentials)
		}, func(body auth.AuthLoginRequest) (any, error) {
			return body.AsAuthCredentialsLoginRequest()
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var body auth.AuthLoginRequest

			fromErr := test.from(&body)
			if fromErr != nil {
				t.Fatal(fromErr)
			}

			want, err := json.Marshal(test.model)
			if err != nil {
				t.Fatal(err)
			}

			got, err := body.MarshalJSON()
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("constructor bytes = %q, %v; want %q", got, err, want)
			}

			decoded, err := test.as(body)
			if err != nil {
				t.Fatal(err)
			}

			got, err = json.Marshal(decoded)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("decoded bytes = %q, %v; want %q", got, err, want)
			}

			request, err := authapi.NewLoginAuthTokenRequest("https://example.invalid", nil, body)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				closeErr := request.Body.Close()
				if closeErr != nil {
					t.Error(closeErr)
				}
			})
			got, err = io.ReadAll(request.Body)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("request bytes = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestLoginUnionRejectsInvalidBodiesWithoutMutation(t *testing.T) {
	t.Parallel()

	const valid = ` {"password":"secret", "appName":null,"apple_id":"account"} `
	for _, invalid := range []string{
		``, `null`, `[]`, `{}`, `{"foreign":true}`,
		`{"appName":null,"apple_id":"account"}`,
		`{"appName":null,"apple_id":null,"password":"secret"}`,
		`{"appName":12,"apple_id":"account","password":"secret"}`,
		`{"appName":null,"apple_id":"account","password":true}`,
		`{"appName":null,"apple_id":"account","password":"secret","unknown":true}`,
		`{"accountCountryCode":null,"dsWebAuthToken":"token","extended_login":"true","trustToken":"trust"}`,
		`{"accountCountryCode":null,"dsWebAuthToken":"token","extended_login":null,"trustToken":"trust"}`,
	} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()

			var body auth.AuthLoginRequest

			decodeErr := body.UnmarshalJSON([]byte(valid))
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}

			invalidErr := body.UnmarshalJSON([]byte(invalid))
			if invalidErr == nil {
				t.Fatal("invalid union accepted")
			}

			got, err := body.MarshalJSON()
			if err != nil || string(got) != valid {
				t.Fatalf("invalid decode mutated bytes = %q, %v", got, err)
			}

			corrupt := auth.CorruptAuthLoginRequest([]byte(invalid))
			if _, err := corrupt.MarshalJSON(); err == nil {
				t.Fatal("invalid private storage serialized")
			}

			request, err := authapi.NewLoginAuthTokenRequest("https://example.invalid", nil, corrupt)
			if err == nil || request != nil {
				t.Fatal("invalid union produced an outbound request")
			}
		})
	}
}
