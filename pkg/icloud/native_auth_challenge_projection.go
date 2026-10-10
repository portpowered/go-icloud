package icloud

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
	bridgemodels "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
)

func nativeProjectBootstrap(challenge *NativeAuthChallenge, data *bridgemodels.BridgeBootstrapDirect) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("copy native bridge bootstrap: %w", err)
	}

	challenge.BridgeBootstrap = encoded
	if data.AuthInitialRoute != nil {
		challenge.AuthInitialRoute = *data.AuthInitialRoute
	}

	if data.HasTrustedDevices != nil {
		challenge.HasTrustedDevices = *data.HasTrustedDevices
	}

	phoneData := nativeBootstrapPhones(challenge, data.TwoSV)
	if phoneData == nil {
		return nil
	}

	encoded, err = json.Marshal(phoneData)
	if err != nil {
		return fmt.Errorf("copy native phone verification: %w", err)
	}

	var phones auth.AuthPhoneNumberVerification

	err = json.Unmarshal(encoded, &phones)
	if err != nil {
		return fmt.Errorf("decode native phone verification: %w", err)
	}

	challenge.PhoneNumbers = []TrustedPhoneNumber{}

	return nativeProjectPhones(challenge, nil, &phones)
}

func nativeProjectPhones(challenge *NativeAuthChallenge, preferred *auth.AuthTrustedPhoneNumber,
	phones *auth.AuthPhoneNumberVerification,
) error {
	values := nativeTrustedPhones(preferred, phones)

	for _, value := range values {
		if value.Id == nil {
			continue
		}

		phone, err := nativeProjectPhone(value)
		if err != nil {
			return err
		}

		challenge.PhoneNumbers = append(challenge.PhoneNumbers, phone)
	}

	return nil
}

func nativeTrustedPhones(preferred *auth.AuthTrustedPhoneNumber,
	phones *auth.AuthPhoneNumberVerification,
) []auth.AuthTrustedPhoneNumber {
	values := []auth.AuthTrustedPhoneNumber{}

	if preferred != nil {
		values = append(values, *preferred)
	}

	if phones != nil {
		if preferred == nil && phones.TrustedPhoneNumber != nil {
			values = append(values, *phones.TrustedPhoneNumber)
		}

		if phones.TrustedPhoneNumbers != nil {
			values = append(values, *phones.TrustedPhoneNumbers...)
		}
	}

	return values
}

func nativeProjectPhone(value auth.AuthTrustedPhoneNumber) (TrustedPhoneNumber, error) {
	encoded, err := json.Marshal(value.Id)
	if err != nil {
		return TrustedPhoneNumber{}, fmt.Errorf("copy trusted phone identity: %w", err)
	}

	var identifier TrustedPhoneNumberID

	err = json.Unmarshal(encoded, &identifier)
	if err != nil {
		return TrustedPhoneNumber{}, fmt.Errorf("decode trusted phone identity: %w", err)
	}

	phone := TrustedPhoneNumber{ID: identifier, Number: "", PushMode: "", NonFTEU: nil}
	if value.NumberWithDialCode != nil {
		phone.Number = *value.NumberWithDialCode
	}

	if value.PushMode != nil {
		phone.PushMode = *value.PushMode
	}

	if value.NonFTEU != nil {
		flag := *value.NonFTEU
		phone.NonFTEU = &flag
	}

	return phone, nil
}

func nativeBootstrapPhones(challenge *NativeAuthChallenge,
	second *bridgemodels.BridgeBootstrapSecondFactor,
) *bridgemodels.BridgePhoneNumberVerification {
	if second == nil {
		return nil
	}

	if second.AuthFactors != nil {
		challenge.AuthFactors = slices.Clone(*second.AuthFactors)
	}

	if second.PhoneNumberVerification != nil {
		return second.PhoneNumberVerification
	}

	if second.BridgeInitiateData != nil {
		return second.BridgeInitiateData.PhoneNumberVerification
	}

	return nil
}
