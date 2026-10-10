package replay_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

func TestSharedPhotosNestedProviderLocation(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t, "fixtures/synthetic/http/photos-upload-shared-assets-1.json")
	response := scenario.Exchanges[1].Response
	data := strings.ReplaceAll(string(contractAuthBody(t, response.Body)), "/synthetic/", "/one/two/")

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString([]byte(data)))
	if err != nil {
		t.Fatal(err)
	}

	response.Body.Value = encoded
	scenario.Exchanges[2].Request.Path = "/one/two/webgetassets"

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.PhotosServiceURL = scenario.Initial.Origin
	auth.SharedPhotosServiceURL = syntheticSharedPhotosOrigin

	result, err := client.ListSharedPhotos(t.Context(), icloud.ListSharedPhotosRequest{
		Auth: auth, Album: "synthetic-stream-0"})
	if err != nil {
		t.Fatal(err, transport.AssertConsumed())
	}

	checkSharedPhotosProjection(t, result.Photos, scenario.Result)
	checkReminderSyncResponses(t, result.Responses, scenario.Exchanges)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}
