package savedlogin_test

import (
	"encoding/json"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/referenceconfig"
)

const explicitNullCountry = `,"account_country":null`

func TestReferenceSessionCountryPresence(t *testing.T) {
	t.Parallel()
	for _, country := range []string{"", explicitNullCountry, `,"account_country":"USA"`} {
		t.Run(country, func(t *testing.T) {
			t.Parallel()
			data := []byte(`{"client_id":"synthetic-client","session_token":"synthetic-token"` + country + `}`)

			var session referenceconfig.ReferenceSessionData

			err := json.Unmarshal(data, &session)
			if err != nil {
				t.Fatal(err)
			}
			if session.ClientID != syntheticClientID || session.Token != syntheticSessionToken {
				t.Fatal("generated reference model changed pinned underscore keys")
			}
			if session.Country.IsSpecified() != (country != "") || session.Country.IsNull() != (country == explicitNullCountry) {
				t.Fatal("country omission and explicit null were collapsed")
			}
			encoded, err := json.Marshal(session) //nolint:gosec // G117: synthetic-token round trip; never exported.
			if err != nil {
				t.Fatal(err)
			}

			var fields map[string]json.RawMessage

			err = json.Unmarshal(encoded, &fields)
			if err != nil {
				t.Fatal(err)
			}
			if _, exists := fields["account_country"]; exists != (country != "") {
				t.Fatal("country presence changed on private-state round trip")
			}
		})
	}
}
