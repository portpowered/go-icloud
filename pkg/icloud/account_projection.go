package icloud

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/portpowered/go-icloud/internal/webtransport"
)

func projectDevices(response *webtransport.DevicesResponse) *GetAccountDevicesResult {
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

func publicMetadata(response *webtransport.BytesResponse) ResponseMetadata {
	return ResponseMetadata{StatusCode: response.Status, Headers: responseHeaders(response.Headers)}
}

func projectFamily(response *webtransport.FamilyResponse) *GetAccountFamilyResult {
	members := make([]AccountFamilyMember, 0)

	if response.Data.FamilyMembers != nil {
		for _, member := range *response.Data.FamilyMembers {
			members = append(members, AccountFamilyMember(member))
		}
	}

	return &GetAccountFamilyResult{Members: members,
		AdditionalMetadata: copyAccountMetadata(response.Data.AdditionalProperties),
		Metadata:           publicMetadata(response.Metadata)}
}

func projectStorage(response *webtransport.StorageResponse) *GetAccountStorageResult {
	media := make([]AccountMediaUsage, 0)

	if response.Data.StorageUsageByMedia != nil {
		for _, usage := range *response.Data.StorageUsageByMedia {
			media = append(media, AccountMediaUsage(usage))
		}
	}

	var quota *AccountQuota

	if response.Data.QuotaStatus != nil {
		projected := AccountQuota(*response.Data.QuotaStatus)
		quota = &projected
	}

	return &GetAccountStorageResult{Usage: AccountStorageUsage(response.Data.StorageUsageInfo), Quota: quota, Media: media,
		AdditionalMetadata: copyAccountMetadata(response.Data.AdditionalProperties),
		Metadata:           publicMetadata(response.Metadata)}
}

func copyAccountMetadata(fields map[string]json.RawMessage) AccountMetadata {
	result := make(AccountMetadata)
	for name, value := range fields {
		result[name] = append(json.RawMessage(nil), value...)
	}

	return result
}
