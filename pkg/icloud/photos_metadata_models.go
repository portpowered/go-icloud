package icloud

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/portpowered/go-icloud/internal/remindersdate"
	"math"
	"strconv"

	"github.com/portpowered/go-icloud/internal/protocol"
)

type photoNormalizer func(json.RawMessage) (json.RawMessage, error)

func photoNormalizeChild(object map[string]json.RawMessage, key string, normalize photoNormalizer) error {
	value, present := object[key]
	if !present {
		return nil
	}

	normalized, err := normalize(value)
	if err != nil {
		return err
	}

	object[key] = normalized

	return nil
}

func photoPlainModel(raw json.RawMessage) (json.RawMessage, error) {
	object, err := photoModelObject(raw)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode photo model: %w", err)
	}

	return encoded, nil
}

func photoBase64Value(raw json.RawMessage) (json.RawMessage, error) {
	var text string

	err := json.Unmarshal(raw, &text)
	if err != nil {
		return nil, fmt.Errorf("decode photo bytes string: %w", err)
	}

	data, err := reminderRelatedBase64(text)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return nil, fmt.Errorf("encode photo bytes: %w", err)
	}

	return encoded, nil
}

func photoValueList(raw json.RawMessage, tag string) (json.RawMessage, error) {
	return photoNormalizeList(raw, func(value json.RawMessage) (json.RawMessage, error) {
		return photoNormalizedValue(tag, value)
	})
}

func photoNormalizeList(raw json.RawMessage, normalize photoNormalizer) (json.RawMessage, error) {
	var values []json.RawMessage

	err := json.Unmarshal(raw, &values)
	if err != nil {
		return nil, fmt.Errorf("decode photo list: %w", err)
	}

	for index, value := range values {
		normalized, err := normalize(value)
		if err != nil {
			return nil, err
		}

		values[index] = normalized
	}

	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode photo list: %w", err)
	}

	return encoded, nil
}

func photoMetadataRecord(raw json.RawMessage) (map[string]json.RawMessage, error) {
	object, err := photoModelObject(raw)
	if err != nil {
		return nil, err
	}

	normalizers := map[string]photoNormalizer{
		protocol.PhotosCKRecordExpirationTime:         photoExpirationValue,
		protocol.PhotosCKRecordCreated:                photoMetadataAudit,
		protocol.PhotosCKRecordModified:               photoMetadataAudit,
		protocol.PhotosCKRecordZoneID:                 photoPlainModel,
		protocol.PhotosCKRecordParent:                 photoPlainModel,
		protocol.PhotosCKRecordStableUrl:              photoPlainModel,
		protocol.PhotosCKRecordShare:                  photoMetadataShare,
		protocol.PhotosCKRecordOwner:                  photoMetadataParticipant,
		protocol.PhotosCKRecordCurrentUserParticipant: photoMetadataParticipant,
		protocol.PhotosCKRecordParticipants:           photoMetadataParticipants,
		protocol.PhotosCKRecordRequesters:             photoMetadataParticipants,
		protocol.PhotosCKRecordBlocked:                photoMetadataParticipants,
		protocol.PhotosCKRecordInvitedPCS:             photoMetadataProtection,
		protocol.PhotosCKRecordSelfAddedPCS:           photoMetadataProtection,
		protocol.PhotosCKRecordChainProtectionInfo:    photoMetadataProtection,
	}
	for key, normalize := range normalizers {
		err := photoNormalizeChild(object, key, normalize)
		if err != nil {
			return nil, err
		}
	}

	return object, nil
}

func photoMetadataChildren(raw json.RawMessage, normalizers map[string]photoNormalizer) (json.RawMessage, error) {
	object, err := photoModelObject(raw)
	if err != nil {
		return nil, err
	}

	for key, normalize := range normalizers {
		err := photoNormalizeChild(object, key, normalize)
		if err != nil {
			return nil, err
		}
	}

	encoded, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("encode nested photo metadata: %w", err)
	}

	return encoded, nil
}

func photoMetadataAudit(raw json.RawMessage) (json.RawMessage, error) {
	return photoMetadataChildren(raw, map[string]photoNormalizer{
		protocol.PhotosCKAuditInfoTimestamp: func(value json.RawMessage) (json.RawMessage, error) {
			return photoTimestampValue(value), nil
		},
	})
}

func photoMetadataShare(raw json.RawMessage) (json.RawMessage, error) {
	return photoMetadataChildren(raw, map[string]photoNormalizer{protocol.PhotosCKShareZoneID: photoPlainModel})
}

func photoMetadataParticipant(raw json.RawMessage) (json.RawMessage, error) {
	return photoMetadataChildren(raw, map[string]photoNormalizer{
		protocol.PhotosCKParticipantUserIdentity:   photoMetadataIdentity,
		protocol.PhotosCKParticipantProtectionInfo: photoMetadataProtection,
	})
}

func photoMetadataParticipants(raw json.RawMessage) (json.RawMessage, error) {
	return photoNormalizeList(raw, photoMetadataParticipant)
}

func photoMetadataIdentity(raw json.RawMessage) (json.RawMessage, error) {
	return photoMetadataChildren(raw, map[string]photoNormalizer{
		protocol.PhotosCKUserIdentityNameComponents: photoPlainModel,
		protocol.PhotosCKUserIdentityLookupInfo:     photoPlainModel,
	})
}

func photoMetadataProtection(raw json.RawMessage) (json.RawMessage, error) {
	return photoMetadataChildren(raw, map[string]photoNormalizer{
		protocol.PhotosCKChainProtectionInfoBytes: photoBase64Value,
	})
}

const photoExpirationMillisThreshold = 100000000000

func photoExpirationValue(raw json.RawMessage) (json.RawMessage, error) {
	value, err := reminderInteger(raw)
	if err != nil {
		return nil, err
	}

	millis := value
	if math.Abs(float64(value)) < photoExpirationMillisThreshold {
		millis *= photoMillisPerSecond
	}

	instant := remindersdate.FromMillis(millis)
	if instant == nil {
		return nil, errPhotoCount
	}

	return json.RawMessage(strconv.FormatInt(instant.Unix(), 10)), nil
}
