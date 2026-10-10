package bridge

import (
	"encoding/hex"
	"errors"
	"fmt"

	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var errWireType = errors.New("unsupported or mismatched protobuf wire type")

// ConnectionMessage serializes the source bootstrap envelope including its signature prefix.
func ConnectionMessage(publicKey, nonce, signature []byte) ([]byte, error) {
	prefix, err := hex.DecodeString(string(models.SignaturePrefixHex))
	if err != nil {
		return nil, fmt.Errorf("signature prefix: %w", err)
	}

	if len(signature) < len(prefix) || string(signature[:len(prefix)]) != string(prefix) {
		signature = append(prefix, signature...)
	}

	message := new(bridgepb.ClientMessage)
	message.Connection = new(bridgepb.ConnectionRequest)
	message.Connection.PublicKey = publicKey
	message.Connection.Nonce = nonce
	message.Connection.Signature = signature
	message.Connection.Expiration = new(bridgepb.Expiration)
	message.Connection.Expiration.Seconds = proto.Uint32(message.GetConnection().GetExpiration().GetSeconds())

	return marshalMessage(message)
}

// SubscriptionMessage serializes every topic in caller order.
func SubscriptionMessage(topics []string) ([]byte, error) {
	message := new(bridgepb.ClientMessage)
	message.Subscription = new(bridgepb.SubscriptionRequest)
	message.Subscription.Topics = topics

	return marshalMessage(message)
}

// AcknowledgementMessage acknowledges a push before its topic is considered.
func AcknowledgementMessage(topic []byte, identifier uint64) ([]byte, error) {
	message := new(bridgepb.ClientMessage)
	message.Acknowledgement = new(bridgepb.Acknowledgement)

	message.Acknowledgement.Topic = append([]byte{}, topic...)
	message.Acknowledgement.MessageId = proto.Uint64(identifier)

	return marshalMessage(message)
}

func marshalMessage(message proto.Message) ([]byte, error) {
	encoded, err := proto.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("encode bridge message: %w", err)
	}

	return encoded, nil
}

// DecodeServerMessage rejects unsupported wire encodings before generated decoding.
func DecodeServerMessage(payload []byte) (*bridgepb.ServerMessage, error) {
	message := new(bridgepb.ServerMessage)

	canonical, err := canonicalFields(payload, message.ProtoReflect().Descriptor())
	if err != nil {
		return nil, &ProtocolError{Stage: "decode", Cause: err}
	}

	err = proto.Unmarshal(canonical, message)
	if err != nil {
		return nil, &ProtocolError{Stage: "decode", Cause: err}
	}

	return message, nil
}

// canonicalFields preserves the first singular value selected by the pinned source
// decoder, while retaining every repeated and unknown field.
func canonicalFields(payload []byte, descriptor protoreflect.MessageDescriptor) ([]byte, error) {
	var canonical []byte

	seen := make(map[protowire.Number]bool)

	for len(payload) != 0 {
		number, wireType, count := protowire.ConsumeTag(payload)
		if count < 0 {
			return nil, fmt.Errorf("decode protobuf tag: %w", protowire.ParseError(count))
		}

		field := descriptor.Fields().ByNumber(number)
		value := payload[count:]

		remaining, err := validateField(value, wireType, field)

		if err != nil {
			return nil, err
		}

		keep := field == nil || field.IsList() || !seen[number]
		if keep {
			encoded, err := canonicalField(value[:len(value)-len(remaining)], wireType, field)
			if err != nil {
				return nil, err
			}

			canonical = protowire.AppendTag(canonical, number, wireType)
			canonical = append(canonical, encoded...)
		}

		seen[number] = true
		payload = remaining
	}

	return canonical, nil
}

func canonicalField(value []byte, wireType protowire.Type, field protoreflect.FieldDescriptor) ([]byte, error) {
	if field == nil || wireType != protowire.BytesType || field.Kind() != protoreflect.MessageKind {
		return value, nil
	}

	message, _ := protowire.ConsumeBytes(value)

	canonical, err := canonicalFields(message, field.Message())

	if err != nil {
		return nil, err
	}

	return protowire.AppendBytes(nil, canonical), nil
}

func validateField(payload []byte, wireType protowire.Type, field protoreflect.FieldDescriptor) ([]byte, error) {
	switch wireType {
	case protowire.VarintType:
		return validateInteger(payload, field)
	case protowire.BytesType:
		return validateBytes(payload, field)
	case protowire.Fixed32Type, protowire.Fixed64Type, protowire.StartGroupType, protowire.EndGroupType:
		return nil, errWireType
	}

	return nil, errWireType
}

func validateInteger(payload []byte, field protoreflect.FieldDescriptor) ([]byte, error) {
	if field != nil && field.Kind() != protoreflect.Uint64Kind && field.Kind() != protoreflect.Uint32Kind {
		return nil, errWireType
	}

	_, count := protowire.ConsumeVarint(payload)
	if count < 0 {
		return nil, fmt.Errorf("decode protobuf integer: %w", protowire.ParseError(count))
	}

	return payload[count:], nil
}

func validateBytes(payload []byte, field protoreflect.FieldDescriptor) ([]byte, error) {
	if field != nil && field.Kind() != protoreflect.MessageKind && field.Kind() != protoreflect.BytesKind {
		return nil, errWireType
	}

	_, count := protowire.ConsumeBytes(payload)
	if count < 0 {
		return nil, fmt.Errorf("decode protobuf bytes: %w", protowire.ParseError(count))
	}

	return payload[count:], nil
}
