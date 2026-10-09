package savedlogin_test

import (
	"encoding/json"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/referenceconfig"
)

func TestReferenceSessionCountryPresence(t *testing.T) {
	t.Parallel()
	for _, country := range []string{"", `,"account_country":null`, `,"account_country":"USA"`} {
		t.Run(country, func(t *testing.T) {
			t.Parallel()
			data := []byte(`{"client_id":"synthetic-client","session_token":"synthetic-token"` + country + `}`)
			var session referenceconfig.ReferenceSessionData
			if err := json.Unmarshal(data, &session); err != nil {
				t.Fatal(err)
			}
			if session.ClientID != "synthetic-client" || session.Token != "synthetic-token" {
				t.Fatal("generated reference model changed pinned underscore keys")
			}
			if session.Country.IsSpecified() != (country != "") || session.Country.IsNull() != (country == `,"account_country":null`) {
				t.Fatal("country omission and explicit null were collapsed")
			}
			encoded, err := json.Marshal(session)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if _, exists := fields["account_country"]; exists != (country != "") {
				t.Fatal("country presence changed on private-state round trip")
			}
		})
	}
}
