package icloud

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

const (
	uuidByteCount   = 16
	uuidVersionMask = 0x0f
	uuidVersion4    = 0x40
	uuidVariantMask = 0x3f
	uuidRFCVariant  = 0x80
)

func (sdk *SDK) randomUUID() (string, error) {
	var raw [uuidByteCount]byte

	_, err := io.ReadFull(sdk.random, raw[:])
	if err != nil {
		return "", fmt.Errorf("read UUID entropy: %w", err)
	}

	raw[6] = raw[6]&uuidVersionMask | uuidVersion4
	raw[8] = raw[8]&uuidVariantMask | uuidRFCVariant
	text := strings.ToUpper(hex.EncodeToString(raw[:]))

	return text[:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:], nil
}
