package securitykey

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"time"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

const apduStatusBytes = 2

const presencePollDelay = 250 * time.Millisecond

func (channel *channel) assertU2F(ctx context.Context, request Request) (Assertion, error) {
	data, err := clientData(request)
	if err != nil {
		return emptyAssertion(), err
	}

	credentials, err := decodeCredentials(request.CredentialIDs)
	if err != nil {
		return emptyAssertion(), err
	}

	if len(credentials) == 0 {
		return emptyAssertion(), keyFailure("U2F credentials", ErrUnsupported)
	}

	clientHash := sha256.Sum256(data)
	rpHash := sha256.Sum256([]byte(request.RelyingPartyID))

	for _, credential := range credentials {
		if len(credential.Id) > int(wire.MaxKeyHandleBytes) {
			continue
		}

		response, err := channel.pollU2F(ctx, clientHash[:], rpHash[:], credential.Id)
		if err != nil {
			if credentialRefusal(err) {
				continue
			}

			return emptyAssertion(), err
		}

		if len(response) <= int(wire.U2FHeaderBytes) {
			return emptyAssertion(), keyFailure("U2F signature", ErrProtocol)
		}

		authData := append(rpHash[:], response[:int(wire.U2FHeaderBytes)]...)

		return projectAssertion(request.RelyingPartyID, data, &credential, &wire.AssertionResponse{
			AuthData:            authData,
			Signature:           response[int(wire.U2FHeaderBytes):],
			Credential:          nil,
			NumberOfCredentials: nil,
			User:                nil,
		})
	}

	return emptyAssertion(), &Error{
		Stage:  "U2F credential",
		Status: int(wire.NoCredentials),
		Cause:  ErrProtocol,
	}
}

func credentialRefusal(err error) bool {
	var failure *Error
	if !errors.As(err, &failure) {
		return false
	}

	return failure.Stage == "APDU" || failure.Stage == stageCTAP || failure.Stage == stageHID
}

func (channel *channel) pollU2F(ctx context.Context, clientHash, rpHash, credential []byte) ([]byte, error) {
	credentialLength := len(credential)
	if credentialLength > math.MaxUint8 {
		return nil, keyFailure("U2F handle", ErrProtocol)
	}

	body := append(append(append([]byte(nil), clientHash...), rpHash...), byte(credentialLength))

	body = append(body, credential...)

	bodyLength := len(body)
	if bodyLength > math.MaxUint16 {
		return nil, keyFailure("APDU length", ErrProtocol)
	}

	apdu := make([]byte, int(wire.APDUOverheadBytes)+len(body))
	apdu[1] = byte(wire.AuthenticateInstruction)
	apdu[2] = byte(wire.MessageCommand)
	binary.BigEndian.PutUint16(apdu[5:], uint16(bodyLength))
	copy(apdu[int(wire.APDUHeaderBytes):], body)

	for {
		response, err := channel.exchange(ctx, byte(wire.MessageCommand), apdu)
		if err != nil {
			return nil, err
		}

		if len(response) < apduStatusBytes {
			return nil, keyFailure("APDU framing", ErrProtocol)
		}

		status := int(binary.BigEndian.Uint16(response[len(response)-apduStatusBytes:]))
		if status == int(wire.APDUSuccess) {
			return response[:len(response)-apduStatusBytes], nil
		}

		if status != int(wire.PresenceRequired) {
			return nil, &Error{
				Stage:  "APDU",
				Status: status,
				Cause:  ErrProtocol,
			}
		}

		err = channel.wait(ctx, presencePollDelay)
		if err != nil {
			return nil, keyFailure("U2F presence", err)
		}
	}
}
