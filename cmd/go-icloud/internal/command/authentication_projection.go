package command

import (
	"fmt"
	"strconv"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/commandmodels"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func visiblePhones(phones []icloud.TrustedPhoneNumber) ([]commandmodels.PhoneChoice, error) {
	result := make([]commandmodels.PhoneChoice, 0, len(phones))

	for _, phone := range phones {
		identifier, err := visiblePhoneID(phone.ID)
		if err != nil {
			return nil, err
		}

		result = append(result, commandmodels.PhoneChoice{ID: identifier, Number: phone.Number})
	}

	return result, nil
}

func visiblePhoneID(identifier icloud.TrustedPhoneNumberID) (string, error) {
	numeric, err := identifier.AsTrustedPhoneNumberID0()
	if err == nil {
		return strconv.Itoa(numeric), nil
	}

	text, err := identifier.AsTrustedPhoneNumberID1()
	if err != nil {
		return "", fmt.Errorf("decode discovered phone identifier: %w", err)
	}

	return text, nil
}
