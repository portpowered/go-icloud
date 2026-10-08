package icloud

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/portpowered/go-icloud/internal/accounttransport"
)

func projectDevices(response *accounttransport.DevicesResponse) *GetAccountDevicesResult {
	devices := make([]AccountDevice, 0, len(response.Data.Devices))
	for _, device := range response.Data.Devices {
		devices = append(devices, AccountDevice(device))
	}

	payments := make([]AccountPaymentMethod, 0)

	if response.Data.PaymentMethods != nil {
		for _, payment := range *response.Data.PaymentMethods {
			payments = append(payments, AccountPaymentMethod(payment))
		}
	}

	additional := make(AccountMetadata)
	for name, value := range response.Data.AdditionalProperties {
		additional[name] = append(json.RawMessage(nil), value...)
	}

	return &GetAccountDevicesResult{
		Devices: devices, PaymentMethods: payments, AdditionalMetadata: additional,
		Metadata: ResponseMetadata{StatusCode: response.Status, Headers: responseHeaders(response.Headers)},
	}
}

func responseHeaders(headers http.Header) []Header {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}

	sort.Strings(names)

	result := make([]Header, 0)

	for _, name := range names {
		for _, value := range headers[name] {
			result = append(result, Header{Name: name, Value: value})
		}
	}

	return result
}
