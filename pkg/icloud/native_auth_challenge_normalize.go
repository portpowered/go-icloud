package icloud

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	bridgemodels "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
)

func nativeNormalizeChallenge(data auth.AuthChallenge) (auth.AuthChallenge, error) {
	route, trusted, factors := nativeChallengeContext(data)

	if data.Direct != nil && data.Direct.TwoSV != nil {
		var err error

		data, err = nativeNormalizeSecondFactor(data, data.Direct.TwoSV)
		if err != nil {
			return data, err
		}
	}

	data.AuthInitialRoute, data.HasTrustedDevices, data.AuthFactors = &route, &trusted, &factors
	if data.PhoneNumberVerification != nil && data.PhoneNumberVerification.TrustedPhoneNumber != nil {
		data.TrustedPhoneNumber = data.PhoneNumberVerification.TrustedPhoneNumber
	}

	return data, nil
}

func nativeNormalizeSecondFactor(data auth.AuthChallenge,
	second *bridgemodels.BridgeBootstrapSecondFactor,
) (auth.AuthChallenge, error) {
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

	return data, nil
}

func nativeChallengeContext(data auth.AuthChallenge) (string, bool, []string) {
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
	if direct == nil {
		return route, trusted, factors
	}

	if direct.AuthInitialRoute != nil {
		route = *direct.AuthInitialRoute
	}

	if direct.HasTrustedDevices != nil {
		trusted = *direct.HasTrustedDevices
	}

	if direct.TwoSV != nil && direct.TwoSV.AuthFactors != nil {
		factors = *direct.TwoSV.AuthFactors
	}

	return route, trusted, factors
}
