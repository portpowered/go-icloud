package icloud

import (
	"encoding/json"
	"fmt"

	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
)

// photoTypedMetadata binds normalization to each generated model at its finite
// internal call sites. Generated extension maps retain provider-owned fields.
func photoTypedMetadata[T any](value any) (json.RawMessage, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode normalized photo model: %w", err)
	}

	var projected T

	err = json.Unmarshal(body, &projected)
	if err != nil {
		return nil, fmt.Errorf("decode normalized photo model: %w", err)
	}

	encoded, err := json.Marshal(projected)
	if err != nil {
		return nil, fmt.Errorf("encode projected photo model: %w", err)
	}

	return encoded, nil
}

func photoZoneModel(raw json.RawMessage) (json.RawMessage, error) {
	return photoPlainModel[cloudkit.CKZoneID](raw)
}

func photoParentModel(raw json.RawMessage) (json.RawMessage, error) {
	return photoPlainModel[cloudkit.CKParent](raw)
}

func photoStableURLModel(raw json.RawMessage) (json.RawMessage, error) {
	return photoPlainModel[cloudkit.CKStableUrl](raw)
}

func photoNameModel(raw json.RawMessage) (json.RawMessage, error) {
	return photoPlainModel[cloudkit.CKNameComponents](raw)
}

func photoLookupModel(raw json.RawMessage) (json.RawMessage, error) {
	return photoPlainModel[cloudkit.CKLookupInfo](raw)
}

func photoParticipantProtection(raw json.RawMessage) (json.RawMessage, error) {
	return photoMetadataProtection[cloudkit.CKParticipantProtectionInfo](raw)
}

func photoPCSModel(raw json.RawMessage) (json.RawMessage, error) {
	return photoMetadataProtection[cloudkit.CKPCSInfo](raw)
}

func photoChainProtection(raw json.RawMessage) (json.RawMessage, error) {
	return photoMetadataProtection[cloudkit.CKChainProtectionInfo](raw)
}
