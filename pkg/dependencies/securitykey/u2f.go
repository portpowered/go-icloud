package securitykey

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

const presencePollDelay = 250 * time.Millisecond

func (channel *channel) assertU2F(ctx context.Context, request Request) (Assertion, error) {
	data, err := clientData(request)
	if err != nil {
		return Assertion{}, err
	}
	credentials, err := decodeCredentials(request.CredentialIDs)
	if err != nil {
		return Assertion{}, err
	}
	if len(credentials) == 0 {
		return Assertion{}, &Error{Stage: "U2F credentials", Cause: ErrUnsupported}
	}
	clientHash := sha256.Sum256(data)
	rpHash := sha256.Sum256([]byte(request.RelyingPartyID))
	for _, credential := range credentials {
		if len(credential.Id) > int(wire.MaxKeyHandleBytes) {
			continue
		}
		response, err := channel.pollU2F(ctx, clientHash[:], rpHash[:], credential.Id)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Assertion{}, err
		}
		if err != nil {
			var failure *Error
			if errors.As(err, &failure) && (failure.Stage == "APDU" || failure.Stage == "CTAP" || failure.Stage == "HID") {
				continue
			}
			return Assertion{}, err
		}
		if len(response) <= int(wire.U2FHeaderBytes) {
			return Assertion{}, &Error{Stage: "U2F signature", Cause: ErrProtocol}
		}
		authData := append(rpHash[:], response[:int(wire.U2FHeaderBytes)]...)
		return projectAssertion(request.RelyingPartyID, data, &credential, &wire.AssertionResponse{AuthData: authData, Signature: response[int(wire.U2FHeaderBytes):]})
	}
	return Assertion{}, &Error{Stage: "U2F credential", Status: int(wire.NoCredentials), Cause: ErrProtocol}
}

func (channel *channel) pollU2F(ctx context.Context, clientHash, rpHash, credential []byte) ([]byte, error) {
	body := append(append(append([]byte(nil), clientHash...), rpHash...), byte(len(credential)))
	body = append(body, credential...)
	apdu := make([]byte, int(wire.APDUOverheadBytes)+len(body))
	apdu[1] = byte(wire.AuthenticateInstruction)
	apdu[2] = byte(wire.MessageCommand)
	binary.BigEndian.PutUint16(apdu[5:], uint16(len(body)))
	copy(apdu[int(wire.APDUHeaderBytes):], body)
	for {
		response, err := channel.exchange(ctx, byte(wire.MessageCommand), apdu)
		if err != nil {
			return nil, err
		}
		if len(response) < 2 {
			return nil, &Error{Stage: "APDU framing", Cause: ErrProtocol}
		}
		status := int(binary.BigEndian.Uint16(response[len(response)-2:]))
		if status == int(wire.APDUSuccess) {
			return response[:len(response)-2], nil
		}
		if status != int(wire.PresenceRequired) {
			return nil, &Error{Stage: "APDU", Status: status, Cause: ErrProtocol}
		}
		if err := channel.wait(ctx, presencePollDelay); err != nil {
			return nil, &Error{Stage: "U2F presence", Cause: err}
		}
	}
}
