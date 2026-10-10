package authapi

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// SCHEMA-11: synthetic controls exercise both primitive object schema members.
func TestLoginUnionPreservesConstructorBytes(t *testing.T) {
	token := auth.AuthTokenLoginRequest{DsWebAuthToken: "token", ExtendedLogin: true, TrustToken: "trust"}
	credentials := auth.AuthCredentialsLoginRequest{AppleId: "account", Password: "password"}

	for _, test := range []struct {
		name  string
		model any
		from  func(*LoginAuthTokenJSONBody) error
	}{
		{"token", token, func(body *LoginAuthTokenJSONBody) error {
			return body.FromExternalRef0AuthTokenLoginRequest(token)
		}},
		{"credentials", credentials, func(body *LoginAuthTokenJSONBody) error {
			return body.FromExternalRef0AuthCredentialsLoginRequest(credentials)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var body LoginAuthTokenJSONBody
			if err := test.from(&body); err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(test.model)
			if err != nil {
				t.Fatal(err)
			}
			got, err := body.MarshalJSON()
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("constructor bytes = %q, %v; want %q", got, err, want)
			}
			request, err := NewLoginAuthTokenRequest("https://example.invalid", nil, LoginAuthTokenJSONRequestBody(body))
			if err != nil {
				t.Fatal(err)
			}
			defer request.Body.Close()
			got, err = io.ReadAll(request.Body)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("request bytes = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestLoginUnionRejectsInvalidBodiesWithoutMutation(t *testing.T) {
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
			var body LoginAuthTokenJSONBody
			if err := body.UnmarshalJSON([]byte(valid)); err != nil {
				t.Fatal(err)
			}
			if err := body.UnmarshalJSON([]byte(invalid)); err == nil {
				t.Fatal("invalid union accepted")
			}
			got, err := body.MarshalJSON()
			if err != nil || string(got) != valid {
				t.Fatalf("invalid decode mutated bytes = %q, %v", got, err)
			}
			corrupt := LoginAuthTokenJSONBody{union: json.RawMessage(invalid)}
			if _, err := corrupt.MarshalJSON(); err == nil {
				t.Fatal("invalid private storage serialized")
			}
			request, err := NewLoginAuthTokenRequest("https://example.invalid", nil, LoginAuthTokenJSONRequestBody(corrupt))
			if err == nil || request != nil {
				t.Fatal("invalid union produced an outbound request")
			}
		})
	}
}
