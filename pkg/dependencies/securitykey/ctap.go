package securitykey

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"

	"github.com/fxamacker/cbor/v2"
	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

const maximumAssertions = 1024

func (channel *channel) call(ctx context.Context, command wire.CTAPCommand, request any, response any) error {
	encoder, err := cbor.CTAP2EncOptions().EncMode()
	if err != nil {
		return &Error{Stage: "CBOR configuration", Cause: err}
	}
	payload := []byte{byte(command)}
	if request != nil {
		encoded, err := encoder.Marshal(request)
		if err != nil {
			return &Error{Stage: "CBOR encode", Cause: err}
		}
		payload = append(payload, encoded...)
	}
	if channel.maxMessageBytes > 0 && len(payload) > channel.maxMessageBytes {
		return &Error{Stage: "CTAP", Status: int(wire.RequestTooLarge), Cause: ErrProtocol}
	}
	result, err := channel.exchange(ctx, byte(wire.CBORCommand), payload)
	if err != nil {
		return err
	}
	if len(result) == 0 {
		return &Error{Stage: "CTAP response", Cause: ErrProtocol}
	}
	if result[0] != byte(wire.Success) {
		return &Error{Stage: "CTAP", Status: int(result[0]), Cause: ErrProtocol}
	}
	decoder, err := (cbor.DecOptions{DupMapKey: cbor.DupMapKeyEnforcedAPF, IndefLength: cbor.IndefLengthForbidden, TagsMd: cbor.TagsForbidden}).DecMode()
	if err != nil {
		return &Error{Stage: "CBOR configuration", Cause: err}
	}
	var raw any
	if err := decoder.Unmarshal(result[1:], &raw); err != nil {
		return &Error{Stage: "CBOR decode", Cause: err}
	}
	canonical, err := encoder.Marshal(raw)
	if err != nil || !bytes.Equal(canonical, result[1:]) {
		return &Error{Stage: "CBOR canonical", Cause: ErrProtocol}
	}
	if err := decoder.Unmarshal(result[1:], response); err != nil {
		return &Error{Stage: "CTAP decode", Cause: err}
	}
	return nil
}

func (channel *channel) info(ctx context.Context) (*wire.InfoResponse, error) {
	var response wire.InfoResponse
	if err := channel.call(ctx, wire.GetInfo, nil, &response); err != nil {
		return nil, err
	}
	if len(response.Versions) == 0 || len(response.Aaguid) != 16 {
		return nil, &Error{Stage: "info", Cause: ErrProtocol}
	}
	return &response, nil
}

func (channel *channel) assert(ctx context.Context, request Request) (Assertion, error) {
	info, err := channel.info(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Assertion{}, err
		}
		var failure *Error
		if !errors.As(err, &failure) || !ctapFallbackFailure(failure.Stage) {
			return Assertion{}, err
		}
		// Fido2Client falls back to CTAP1 when CTAP2 initialization fails.
		return channel.assertU2F(ctx, request)
	}
	channel.maxMessageBytes = int(wire.DefaultMaxMessageBytes)
	if info.MaxMsgSize != nil {
		if *info.MaxMsgSize < 1 {
			return Assertion{}, &Error{Stage: "info message limit", Cause: ErrProtocol}
		}
		channel.maxMessageBytes = *info.MaxMsgSize
	}
	data, err := clientData(request)
	if err != nil {
		return Assertion{}, err
	}
	credentials, err := decodeCredentials(request.CredentialIDs)
	if err != nil {
		return Assertion{}, err
	}
	protocol := negotiatedProtocol(info)
	required, allowUV := false, true
	for {
		result, err := channel.assertAttempt(ctx, request, data, credentials, protocol, required, allowUV)
		var failure *Error
		if !errors.As(err, &failure) || failure.Stage != "CTAP" {
			return result, err
		}
		switch failure.Status {
		case int(wire.TokenRequired):
			if required {
				return result, err
			}
			required = true
		case int(wire.UVBlocked):
			if !allowUV {
				return result, err
			}
			allowUV = false
		default:
			return result, err
		}
	}
}

func ctapFallbackFailure(stage string) bool {
	switch stage {
	case "CTAP", "CBOR decode", "CBOR canonical", "CTAP decode", "info":
		return true
	default:
		return false
	}
}

func (channel *channel) assertAttempt(ctx context.Context, request Request, data []byte, credentials []wire.Credential, protocol wire.PINProtocol, required, allowUV bool) (Assertion, error) {
	info, err := channel.info(ctx)
	if err != nil {
		return Assertion{}, err
	}
	parameters, err := channel.authParameters(ctx, info, protocol, request.RelyingPartyID, required, allowUV)
	if err != nil {
		return Assertion{}, err
	}
	selected, err := channel.selectCredential(ctx, request.RelyingPartyID, credentials, parameters)
	if err != nil {
		return Assertion{}, err
	}
	hash := sha256.Sum256(data)
	assertionRequest := wire.AssertionRequest{RpId: request.RelyingPartyID, ClientDataHash: hash[:]}
	parameters.apply(&assertionRequest)
	if parameters.internalUV {
		verified := true
		assertionRequest.Options = &wire.AssertionOptions{Uv: &verified}
	}
	if selected != nil {
		assertionRequest.AllowList = &[]wire.Credential{*selected}
	}
	response, err := channel.assertions(ctx, assertionRequest)
	if err != nil {
		return Assertion{}, err
	}
	result, err := projectAssertion(request.RelyingPartyID, data, selected, response)
	if err == nil && parameters.internalUV && result.AuthenticatorData[sha256.Size]&byte(wire.UserVerificationFlag) == 0 {
		return Assertion{}, &Error{Stage: "UV assertion", Cause: ErrProtocol}
	}
	return result, err
}

func clientData(request Request) ([]byte, error) {
	origin, parseErr := url.Parse(request.Origin)
	if parseErr != nil || origin.Scheme != "https" || !strings.Contains(request.RelyingPartyID, ".") || (origin.Hostname() != request.RelyingPartyID && !strings.HasSuffix(origin.Hostname(), "."+request.RelyingPartyID)) {
		return nil, &Error{Stage: "origin binding", Cause: ErrProtocol}
	}
	suffix, _ := publicsuffix.PublicSuffix(request.RelyingPartyID)
	if suffix == request.RelyingPartyID {
		return nil, &Error{Stage: "relying party suffix", Cause: ErrProtocol}
	}
	challenge, err := decodeBase64URL(request.Challenge)
	if err != nil || len(challenge) == 0 {
		return nil, &Error{Stage: "challenge", Cause: ErrProtocol}
	}
	// Python's CollectedClientData has this stable field order. Generated models sort fields,
	// so serialize the generated values individually into the prescribed wire sequence.
	value := wire.ClientData{Type: wire.WebauthnGet, Challenge: base64.RawURLEncoding.EncodeToString(challenge), Origin: request.Origin, CrossOrigin: false}
	kind, _ := json.Marshal(value.Type)
	encodedChallenge, _ := json.Marshal(value.Challenge)
	encodedOrigin, err := json.Marshal(value.Origin)
	if err != nil {
		return nil, &Error{Stage: "client data", Cause: err}
	}
	return fmt.Appendf(nil, string(wire.CollectedClientDataTemplate), kind, encodedChallenge, encodedOrigin), nil
}

func decodeCredentials(ids []string) ([]wire.Credential, error) {
	credentials := make([]wire.Credential, 0, len(ids))
	for _, id := range ids {
		decoded, err := decodeBase64URL(id)
		if err != nil || len(decoded) == 0 {
			return nil, &Error{Stage: "credential", Cause: ErrProtocol}
		}
		credentials = append(credentials, wire.Credential{Id: decoded, Type: wire.PublicKey})
	}
	return credentials, nil
}

func decodeBase64URL(value string) ([]byte, error) {
	if strings.HasSuffix(value, "=") {
		return base64.URLEncoding.DecodeString(value)
	}
	return base64.RawURLEncoding.DecodeString(value)
}

func (channel *channel) assertions(ctx context.Context, request wire.AssertionRequest) (*wire.AssertionResponse, error) {
	var response wire.AssertionResponse
	if err := channel.call(ctx, wire.GetAssertion, request, &response); err != nil {
		return nil, err
	}
	count := 1
	if response.NumberOfCredentials != nil {
		count = *response.NumberOfCredentials
	}
	if count < 1 || count > maximumAssertions {
		return nil, &Error{Stage: "assertion count", Cause: ErrProtocol}
	}
	for range count - 1 {
		var next wire.AssertionResponse
		if err := channel.call(ctx, wire.GetNextAssertion, nil, &next); err != nil {
			return nil, err
		}
	}
	return &response, nil
}

func (channel *channel) selectCredential(ctx context.Context, rp string, credentials []wire.Credential, parameters authParameters) (*wire.Credential, error) {
	if len(credentials) == 0 {
		return nil, nil
	}
	info, err := channel.info(ctx)
	if err != nil {
		return nil, err
	}
	maximum := 1
	if info.MaxCredentialCountInList != nil && *info.MaxCredentialCountInList > 0 {
		maximum = *info.MaxCredentialCountInList
	}
	filtered := []wire.Credential{}
	for _, credential := range credentials {
		if info.MaxCredentialIdLength == nil || *info.MaxCredentialIdLength == 0 || len(credential.Id) <= *info.MaxCredentialIdLength {
			filtered = append(filtered, credential)
		}
	}
	for len(filtered) > 0 {
		count := min(maximum, len(filtered))
		chunk := filtered[:count]
		presence := false
		request := wire.AssertionRequest{RpId: rp, ClientDataHash: make([]byte, sha256.Size), AllowList: &chunk, Options: &wire.AssertionOptions{Up: &presence}}
		parameters.apply(&request)
		response, err := channel.assertions(ctx, request)
		if err == nil {
			if count == 1 {
				return &chunk[0], nil
			}
			if response.Credential == nil {
				return nil, &Error{Stage: "credential selection", Cause: ErrProtocol}
			}
			for _, candidate := range chunk {
				if bytes.Equal(candidate.Id, response.Credential.Id) {
					return response.Credential, nil
				}
			}
			return nil, &Error{Stage: "credential selection", Cause: ErrProtocol}
		}
		failure, ok := err.(*Error)
		if !ok || failure.Stage != "CTAP" {
			return nil, err
		}
		switch failure.Status {
		case int(wire.RequestTooLarge):
			if maximum <= 1 {
				return nil, err
			}
			maximum--
		case int(wire.NoCredentials):
			filtered = filtered[count:]
		default:
			return nil, err
		}
	}
	return &wire.Credential{Id: []byte{0}, Type: wire.PublicKey}, nil
}

func projectAssertion(rp string, data []byte, credential *wire.Credential, response *wire.AssertionResponse) (Assertion, error) {
	hash := sha256.Sum256([]byte(rp))
	if len(response.AuthData) < int(wire.AuthenticatorHeaderBytes) || !bytes.Equal(response.AuthData[:sha256.Size], hash[:]) || response.AuthData[sha256.Size]&byte(wire.UserPresenceFlag) == 0 || len(response.Signature) == 0 {
		return Assertion{}, &Error{Stage: "assertion binding", Cause: ErrProtocol}
	}
	selected := response.Credential
	if selected == nil {
		selected = credential
	}
	if selected == nil || selected.Type != wire.PublicKey || len(selected.Id) == 0 || (credential != nil && !bytes.Equal(selected.Id, credential.Id)) {
		return Assertion{}, &Error{Stage: "assertion credential", Cause: ErrProtocol}
	}
	result := Assertion{ClientData: data, AuthenticatorData: response.AuthData, Signature: response.Signature, CredentialID: append([]byte(nil), selected.Id...)}
	if response.User != nil && response.User.Id != nil {
		result.UserHandle = append([]byte(nil), (*response.User.Id)...)
	}
	return result, nil
}
