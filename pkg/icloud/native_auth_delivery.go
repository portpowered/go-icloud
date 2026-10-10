package icloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencies/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

// RequestTwoFactorCode requests SMS delivery or returns a security-key/explicit bridge route.
func (sdk *SDK) RequestTwoFactorCode(ctx context.Context,
	request RequestTwoFactorCodeRequest,
) (*NativeAuthResult, error) {
	operation, err := newNativeAuthOperation(ctx, "RequestTwoFactorCode", request.Auth, request.State)
	if err != nil {
		return nil, err
	}

	err = sdk.nativeRequestCode(ctx, operation, request.PhoneNumberID)
	if err != nil {
		return nil, err
	}

	return nativeAuthResult(operation)
}

func (sdk *SDK) nativeRequestCode(ctx context.Context, operation *nativeAuthOperation,
	selected *TrustedPhoneNumberID,
) error {
	challenge := operation.state.Challenge

	if nativeSkipCodeDelivery(operation, selected) {
		return nil
	}

	phone, err := nativeSelectedPhone(challenge, selected)
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	if !nativeSMSDelivery(challenge, phone) {
		operation.success = false

		return nil
	}

	data, err := nativePhonePayload(phone)
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	input := auth.AuthSMSRequest{PhoneNumber: data, Mode: auth.Sms}

	request, err := nativeEncodedRequest(webtransport.RequestAuthSMSCall{
		Origin: nativeIDMSOrigin(operation.state), Params: nil, Body: input,
	})
	if err != nil {
		return newClientError(operation.name, Configuration, 0, nil, nil, err)
	}

	response, err := sdk.nativeAuthExchange(ctx, operation, request,
		nativeAuthHeaders(operation.state, nativeJSONMedia()))
	if err != nil {
		return err
	}

	err = nativeRequireSuccess(operation, response)
	if err != nil {
		return err
	}

	prioritized, err := nativePrioritizePhone(challenge.PhoneNumbers, phone)
	if err != nil {
		return nativeResponseError(operation, response, err, InvalidResponse)
	}

	operation.state.Challenge.PhoneNumbers = prioritized
	if selected == nil {
		operation.state.DeliveryNotice = nil
	}

	operation.state.DeliveryMethod = TwoFactorDeliverySMS
	operation.state.CodeRequested = true

	return nil
}

func nativeSelectedPhone(challenge NativeAuthChallenge, selected *TrustedPhoneNumberID) (TrustedPhoneNumber, error) {
	if len(challenge.PhoneNumbers) == 0 {
		return TrustedPhoneNumber{}, errNativeAuthInput
	}

	if selected == nil {
		return challenge.PhoneNumbers[0], nil
	}

	wanted, err := json.Marshal(selected)
	if err != nil {
		return TrustedPhoneNumber{}, fmt.Errorf("encode selected phone: %w", err)
	}

	for _, phone := range challenge.PhoneNumbers {
		actual, encodeErr := json.Marshal(phone.ID)
		if encodeErr != nil {
			return TrustedPhoneNumber{}, fmt.Errorf("encode trusted phone: %w", encodeErr)
		}

		if bytes.Equal(wanted, actual) {
			return phone, nil
		}
	}

	return TrustedPhoneNumber{}, errNativeAuthInput
}

func nativePhonePayload(phone TrustedPhoneNumber) (auth.AuthPhoneNumber, error) {
	encoded, err := json.Marshal(phone.ID)
	if err != nil {
		return auth.AuthPhoneNumber{}, fmt.Errorf("encode phone identity: %w", err)
	}

	var identifier auth.AuthPhoneID

	err = json.Unmarshal(encoded, &identifier)
	if err != nil {
		return auth.AuthPhoneNumber{}, fmt.Errorf("decode phone identity: %w", err)
	}

	return auth.AuthPhoneNumber{Id: identifier, NonFTEU: phone.NonFTEU, AdditionalProperties: nil}, nil
}

func nativePrioritizePhone(phones []TrustedPhoneNumber, selected TrustedPhoneNumber) ([]TrustedPhoneNumber, error) {
	result := []TrustedPhoneNumber{selected}

	wanted, err := json.Marshal(selected.ID)
	if err != nil {
		return nil, fmt.Errorf("encode selected phone identity: %w", err)
	}

	for _, phone := range phones {
		actual, err := json.Marshal(phone.ID)
		if err != nil {
			return nil, fmt.Errorf("encode prioritized phone identity: %w", err)
		}

		if !bytes.Equal(wanted, actual) {
			result = append(result, phone)
		}
	}

	return result, nil
}

func nativeSkipCodeDelivery(operation *nativeAuthOperation, selected *TrustedPhoneNumberID) bool {
	challenge := operation.state.Challenge
	if challenge.SecurityKeyChallenge != nil || len(challenge.SecurityKeyNames) > 0 {
		operation.state.DeliveryMethod = TwoFactorDeliverySecurityKey
		operation.state.DeliveryNotice = nil
		operation.success = false

		return true
	}

	if operation.state.CodeRequested {
		return true
	}

	if selected == nil && challenge.AuthInitialRoute == protocol.AuthBridgeInitialRouteValue &&
		challenge.HasTrustedDevices &&
		len(challenge.BridgeBootstrap) > 0 {
		operation.state.DeliveryMethod = TwoFactorDeliveryTrustedDevice
		operation.state.DeliveryNotice = nil
		operation.success = false

		return true
	}

	if len(challenge.PhoneNumbers) == 0 && selected == nil {
		operation.success = false

		return true
	}

	return false
}

func nativeSMSDelivery(challenge NativeAuthChallenge, phone TrustedPhoneNumber) bool {
	return (challenge.Mode == "" || challenge.Mode == string(auth.Sms)) &&
		(phone.PushMode == string(auth.Sms) || challenge.Mode != "")
}
