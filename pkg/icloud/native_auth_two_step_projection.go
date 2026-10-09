package icloud

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/auth"
)

func nativeProjectDevices(data auth.AuthTrustedDevicesResponse) ([]TrustedAuthDevice, error) {
	result := []TrustedAuthDevice{}
	if data.Devices == nil {
		return result, nil
	}
	for _, device := range *data.Devices {
		metadata, err := json.Marshal(device)
		if err != nil {
			return nil, fmt.Errorf("copy trusted device: %w", err)
		}
		identifier := ""
		if device.Id != nil {
			identifier = *device.Id
		}
		result = append(result, TrustedAuthDevice{ID: identifier, Metadata: metadata})
	}
	return result, nil
}

func nativeDevicePayload(device TrustedAuthDevice) (auth.AuthTrustedDevice, error) {
	var result auth.AuthTrustedDevice
	if len(device.Metadata) > 0 {
		if err := json.Unmarshal(device.Metadata, &result); err != nil {
			return result, fmt.Errorf("decode trusted device: %w", err)
		}
	}
	if result.Id == nil {
		result.Id = &device.ID
	}
	if *result.Id != device.ID || device.ID == "" {
		return result, errNativeAuthInput
	}
	return result, nil
}

func nativeWrongTwoStepResult(operation *nativeAuthOperation, err error) (*NativeAuthResult, error) {
	var failure *ClientError
	if !errors.As(err, &failure) {
		return nil, err
	}
	var data auth.AuthFailure
	if json.Unmarshal(failure.ResponseBody(), &data) != nil || data.ErrorCode == nil {
		return nil, err
	}
	code, decodeErr := data.ErrorCode.AsAuthFailureErrorCode0()
	if decodeErr != nil || code != int(auth.WrongVerificationCode) {
		return nil, err
	}
	operation.success = false
	return nativeAuthResult(operation)
}
