package webtransport

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
)

// clearFindMyTheftLoss changes only the known top-level value, keeping provider member order.
// The schema-owned raw type avoids sorting an opaque context through a Go map.
func clearFindMyTheftLoss(context findmy.FindMyRefreshContext) (findmy.FindMyRefreshContext, error) {
	_, err := accountFields(context)
	if err != nil {
		return nil, fmt.Errorf("decode Find My refresh context: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(context))

	_, err = decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("open Find My refresh context: %w", err)
	}

	start, end := 0, 0

	for decoder.More() {
		key, tokenErr := decoder.Token()
		if tokenErr != nil {
			return nil, fmt.Errorf("read Find My context key: %w", tokenErr)
		}

		var value json.RawMessage

		decodeErr := decoder.Decode(&value)
		if decodeErr != nil {
			return nil, fmt.Errorf("read Find My context value: %w", decodeErr)
		}

		if key == protocol.FindMyServerContextTheftLoss {
			end = int(decoder.InputOffset())
			start = end - len(value)
		}
	}

	result := append(findmy.FindMyRefreshContext(nil), context...)
	if end != 0 {
		result = append(append(append(findmy.FindMyRefreshContext(nil), context[:start]...),
			json.RawMessage("null")...), context[end:]...)
	}

	return result, nil
}
