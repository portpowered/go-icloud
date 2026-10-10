package bridge

import (
	"bytes"
	"context"
	"crypto/sha1" // #nosec G505 -- APNS topic identity is protocol-mandated SHA-1.
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
)

const (
	tokenStage = "token"
	pushStage  = "push"
)

var (
	errPushToken    = errors.New("missing or rejected bridge push token")
	errPushPayload  = errors.New("invalid bridge push payload")
	errSubscription = errors.New("bridge topic subscription rejected")
)

// Socket is the cancellable, connection-producing seam for every framed exchange.
type Socket interface {
	SendBinary(ctx context.Context, payload []byte) error
	ReadMessage(ctx context.Context) ([]byte, error)
	Close() error
}

// Push contains the generated full envelope and its normalized session and step.
type Push struct {
	Payload   models.BridgePushPayload
	SessionID string
	NextStep  string
}

// WaitOptions supplies the bounded wait and a deterministic monotonic clock.
type WaitOptions struct {
	Timeout time.Duration
	Clock   func() time.Time
}

// WaitPushToken ignores unrelated message variants, preserving nonce retry evidence.
func WaitPushToken(ctx context.Context, socket Socket, options WaitOptions) ([]byte, error) {
	if options.Clock == nil {
		options.Clock = time.Now
	}

	work, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	deadline := options.Clock().Add(options.Timeout)
	for options.Clock().Before(deadline) {
		message, err := readServer(work, socket)
		if err != nil {
			return nil, err
		}

		if message.GetConnection() == nil {
			continue
		}

		return decodeToken(message.GetConnection())
	}

	return nil, &ProtocolError{Stage: tokenStage, Cause: context.DeadlineExceeded}
}

func decodeToken(response *bridgepb.ConnectionResponse) ([]byte, error) {
	if response.GetStatus() == uint64(bridgepb.Status_STATUS_INVALID_NONCE) && response.ServerTimestampSeconds != nil {
		milliseconds := response.GetServerTimestampSeconds() * uint64(time.Second/time.Millisecond)

		return nil, &InvalidNonceError{TimestampMilliseconds: milliseconds}
	}

	if response.GetStatus() != uint64(bridgepb.Status_STATUS_OK) || len(response.GetPushTokenBase64()) == 0 {
		return nil, &ProtocolError{Stage: tokenStage, Cause: errPushToken}
	}

	value, err := strictBase64(string(response.GetPushTokenBase64()))
	if err != nil {
		return nil, &ProtocolError{Stage: tokenStage, Cause: err}
	}

	return value, nil
}

// WaitPush acknowledges every delivery, ignores other topics, and validates the full relevant envelope.
func WaitPush(ctx context.Context, socket Socket, topic string, options WaitOptions) (*Push, error) {
	if options.Clock == nil {
		options.Clock = time.Now
	}

	work, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	deadline := options.Clock().Add(options.Timeout)
	digest := sha1.Sum([]byte(topic)) // #nosec G401 -- APNS topic identity requires SHA-1.

	for options.Clock().Before(deadline) {
		message, err := readServer(work, socket)
		if err != nil {
			return nil, err
		}

		if message.GetSubscription() != nil && message.GetSubscription().GetStatus() != uint64(bridgepb.Status_STATUS_OK) {
			return nil, &ProtocolError{Stage: "subscription", Cause: errSubscription}
		}

		if message.GetPush() == nil {
			continue
		}

		err = acknowledge(work, socket, message.GetPush())
		if err != nil {
			return nil, err
		}

		namedTopic := string(bytes.ToValidUTF8(message.GetPush().GetTopic(), nil))
		if namedTopic != topic && !bytes.Equal(message.GetPush().GetTopic(), digest[:]) {
			continue
		}

		return DecodePush(message.GetPush().GetPayload())
	}

	return nil, &ProtocolError{Stage: pushStage, Cause: context.DeadlineExceeded}
}

func readServer(ctx context.Context, socket Socket) (*bridgepb.ServerMessage, error) {
	payload, err := socket.ReadMessage(ctx)
	if err != nil {
		return nil, &ProtocolError{Stage: "receive", Cause: err}
	}

	return DecodeServerMessage(payload)
}

func acknowledge(ctx context.Context, socket Socket, push *bridgepb.PushMessage) error {
	payload, err := AcknowledgementMessage(push.GetTopic(), push.GetMessageId())
	if err != nil {
		return err
	}

	err = socket.SendBinary(ctx, payload)
	if err != nil {
		return &ProtocolError{Stage: "acknowledge", Cause: err}
	}

	return nil
}

// DecodePush extracts an embedded JSON object without losing unknown server members.
func DecodePush(payload []byte) (*Push, error) {
	object := extractObject(payload)
	if object == nil {
		return nil, &ProtocolError{Stage: pushStage, Cause: errPushPayload}
	}

	var generated models.BridgePushPayload

	err := json.Unmarshal(object, &generated)
	if err != nil {
		return nil, &ProtocolError{Stage: pushStage, Cause: err}
	}

	err = validatePush(generated)
	if err != nil {
		return nil, &ProtocolError{Stage: pushStage, Cause: err}
	}

	identifier := valueOrEmpty(generated.SessionUUID)
	if identifier == "" {
		identifier = valueOrEmpty(generated.Flowid)
	}

	step, err := nextStep(generated.NextStep)
	if err != nil {
		return nil, &ProtocolError{Stage: pushStage, Cause: err}
	}

	return &Push{Payload: generated, SessionID: identifier, NextStep: step}, nil
}

func extractObject(payload []byte) []byte {
	payload = bytes.ToValidUTF8(payload, nil)
	if json.Valid(payload) {
		if bytes.HasPrefix(bytes.TrimSpace(payload), []byte("{")) {
			return payload
		}

		return nil
	}

	for start := bytes.IndexByte(payload, '{'); start >= 0; {
		decoder := json.NewDecoder(bytes.NewReader(payload[start:]))

		var object json.RawMessage

		if decoder.Decode(&object) == nil && bytes.HasPrefix(object, []byte("{")) {
			return object
		}

		next := bytes.IndexByte(payload[start+1:], '{')
		if next < 0 {
			break
		}

		start += next + 1
	}

	return nil
}

func validatePush(payload models.BridgePushPayload) error {
	values := []*string{payload.SessionUUID, payload.Flowid, payload.Txnid, payload.Salt, payload.Mid,
		payload.Idmsdata, payload.Data, payload.EncryptedCode}
	for _, value := range values {
		if value != nil && strings.TrimSpace(*value) == "" {
			return errPushPayload
		}
	}

	if payload.SessionUUID == nil && payload.Flowid == nil {
		return errPushPayload
	}

	return nil
}

func nextStep(step *models.BridgePushPayload_NextStep) (string, error) {
	if step == nil {
		return "", nil
	}

	text, err := step.AsBridgePushPayloadNextStep0()
	if err == nil {
		if strings.TrimSpace(text) == "" {
			return "", errPushPayload
		}

		return text, nil
	}

	number, err := step.AsBridgePushPayloadNextStep1()
	if err != nil {
		return "", fmt.Errorf("decode next step: %w", err)
	}

	return strconv.Itoa(number), nil
}

func strictBase64(value string) ([]byte, error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, errPushPayload
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode bridge base64: %w", err)
	}

	return decoded, nil
}

// TopicHash returns the source-compatible lower-case SHA-1 topic identity.
func TopicHash(topic string) string {
	digest := sha1.Sum([]byte(topic)) // #nosec G401 -- APNS topic identity requires SHA-1.

	return hex.EncodeToString(digest[:])
}
