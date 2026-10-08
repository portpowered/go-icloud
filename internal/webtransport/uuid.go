package webtransport

import (
	"crypto/rand"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
)

const (
	uuidByteCount         = 16
	uuidVersionIndex      = 6
	uuidVariantIndex      = 8
	uuidVersionMask  byte = 0x0f
	uuidVariantMask  byte = 0x3f
	uuidVersionFour  byte = 0x40
	uuidRFCVariant   byte = 0x80
	uuidFirstEnd          = 4
	uuidSecondEnd         = 6
	uuidThirdEnd          = 8
	uuidFourthEnd         = 10
)

func temporaryFolderID() (string, error) {
	value := make([]byte, uuidByteCount)

	_, err := rand.Read(value)
	if err != nil {
		return "", fmt.Errorf("create temporary folder identifier: %w", err)
	}

	value[uuidVersionIndex] = value[uuidVersionIndex]&uuidVersionMask | uuidVersionFour
	value[uuidVariantIndex] = value[uuidVariantIndex]&uuidVariantMask | uuidRFCVariant

	return fmt.Sprintf("%s%x-%x-%x-%x-%x", protocol.DriveFolderCreationClientIdPrefix,
		value[:uuidFirstEnd], value[uuidFirstEnd:uuidSecondEnd], value[uuidSecondEnd:uuidThirdEnd],
		value[uuidThirdEnd:uuidFourthEnd], value[uuidFourthEnd:]), nil
}
