package icloud

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

func nativeNormalizeChallenge(data auth.AuthChallenge) (auth.AuthChallenge, error) {
	route, trusted, factors := "", false, []string{}
	if data.AuthInitialRoute != nil {
		route = *data.AuthInitialRoute
	}
	if data.HasTrustedDevices != nil {
		trusted = *data.HasTrustedDevices
	}
	if data.AuthFactors != nil {
		factors = *data.AuthFactors
	}
	direct := data.Direct
	if direct != nil {
		if direct.AuthInitialRoute != nil {
			route = *direct.AuthInitialRoute
		}
		if direct.HasTrustedDevices != nil {
			trusted = *direct.HasTrustedDevices
		}
		if direct.TwoSV != nil {
			second := direct.TwoSV
			if second.AuthFactors != nil {
				factors = *second.AuthFactors
			}
			if second.BridgeInitiateData != nil {
				data.BridgeInitiateData = second.BridgeInitiateData
			}
			phone := second.PhoneNumberVerification
			if phone == nil && second.BridgeInitiateData != nil {
				phone = second.BridgeInitiateData.PhoneNumberVerification
			}
			if phone != nil {
				encoded, err := json.Marshal(phone)
				if err != nil {
					return data, fmt.Errorf("encode normalized phone verification: %w", err)
				}
				var verification auth.AuthPhoneNumberVerification
				err = json.Unmarshal(encoded, &verification)
				if err != nil {
					return data, fmt.Errorf("decode normalized phone verification: %w", err)
				}
				data.PhoneNumberVerification = &verification
			}
			if second.SourceAppId != nil {
				encoded, err := json.Marshal(second.SourceAppId)
				if err != nil {
					return data, fmt.Errorf("encode normalized application identity: %w", err)
				}
				var identity auth.AuthChallenge_SourceAppId
				err = json.Unmarshal(encoded, &identity)
				if err != nil {
					return data, fmt.Errorf("decode normalized application identity: %w", err)
				}
				data.SourceAppId = &identity
			}
		}
	}
	data.AuthInitialRoute, data.HasTrustedDevices, data.AuthFactors = &route, &trusted, &factors
	if data.PhoneNumberVerification != nil && data.PhoneNumberVerification.TrustedPhoneNumber != nil {
		data.TrustedPhoneNumber = data.PhoneNumberVerification.TrustedPhoneNumber
	}
	return data, nil
}
