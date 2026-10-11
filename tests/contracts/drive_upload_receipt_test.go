package contracts_test

import (
	"testing"
)

func TestDriveStructuredRegistrationContracts(t *testing.T) {
	t.Parallel()

	documents := loadDriveDocuments(t)

	for _, variant := range []string{"structured", "rejected", "missing-code"} {
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			exchanges := accountExchanges(t, "../replay/fixtures/sdk/drive-upload-"+variant+".json")

			for index, exchange := range exchanges {
				operation, err := bindDriveOperation(documents, exchanges, index)
				if err != nil {
					t.Fatal(err)
				}

				validateDriveRequest(t, operation, exchange.Request)
				validateDriveResponse(t, operation, exchange.Response)
			}
		})
	}
}
