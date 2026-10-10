package securitykey

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
		return keyFailure("CBOR configuration", err)
	}

	commandByte, err := checkedCommand(command)
	if err != nil {
		return err
	}

	payload := []byte{commandByte}

	if request != nil {
		encoded, err := encoder.Marshal(request)
		if err != nil {
			return keyFailure("CBOR encode", err)
		}

		payload = append(payload, encoded...)
	}

	if channel.maxMessageBytes > 0 && len(payload) > channel.maxMessageBytes {
		return &Error{
			Stage:  stageCTAP,
			Status: int(wire.RequestTooLarge),
			Cause:  ErrProtocol,
		}
	}

	result, err := channel.exchange(ctx, byte(wire.CBORCommand), payload)
	if err != nil {
		return err
	}

	if len(result) == 0 {
		return keyFailure("CTAP response", ErrProtocol)
	}

	if result[0] != byte(wire.Success) {
		return &Error{
			Stage:  stageCTAP,
			Status: int(result[0]),
			Cause:  ErrProtocol,
		}
	}

	return decodeResponse(encoder, result[1:], response)
}

func decodeResponse(encoder cbor.EncMode, encoded []byte, response any) error {
	decoder, err := assertionDecoderOptions().DecMode()
	if err != nil {
		return keyFailure("CBOR configuration", err)
	}

	var raw any

	err = decoder.Unmarshal(encoded, &raw)
	if err != nil {
		return keyFailure("CBOR decode", err)
	}

	canonical, err := encoder.Marshal(raw)
	if err != nil || !bytes.Equal(canonical, encoded) {
		return keyFailure("CBOR canonical", ErrProtocol)
	}

	err = decoder.Unmarshal(encoded, response)
	if err != nil {
		return keyFailure("CTAP decode", err)
	}

	return nil
}

func (channel *channel) info(ctx context.Context) (*wire.InfoResponse, error) {
	var response wire.InfoResponse

	err := channel.call(ctx, wire.GetInfo, nil, &response)
	if err != nil {
		return nil, err
	}

	if len(response.Versions) == 0 || len(response.Aaguid) != 16 {
		return nil, keyFailure("info", ErrProtocol)
	}

	return &response, nil
}

func (channel *channel) assert(ctx context.Context, request Request) (Assertion, error) {
	info, err := channel.info(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return emptyAssertion(), err
		}

		var failure *Error
		if !errors.As(err, &failure) || !ctapFallbackFailure(failure.Stage) {
			return emptyAssertion(), err
		}
		// Fido2Client falls back to CTAP1 when CTAP2 initialization fails.
		return channel.assertU2F(ctx, request)
	}

	err = channel.setMessageLimit(info)
	if err != nil {
		return emptyAssertion(), err
	}

	data, err := clientData(request)
	if err != nil {
		return emptyAssertion(), err
	}

	credentials, err := decodeCredentials(request.CredentialIDs)
	if err != nil {
		return emptyAssertion(), err
	}

	return channel.retryAssertion(ctx, request, data, credentials, negotiatedProtocol(info))
}

func (channel *channel) setMessageLimit(info *wire.InfoResponse) error {
	channel.maxMessageBytes = int(wire.DefaultMaxMessageBytes)

	if info.MaxMsgSize != nil {
		if *info.MaxMsgSize < 1 {
			return keyFailure("info message limit", ErrProtocol)
		}

		channel.maxMessageBytes = *info.MaxMsgSize
	}

	return nil
}

func (channel *channel) retryAssertion(
	ctx context.Context, request Request, data []byte, credentials []wire.Credential, protocol pinProtocolNegotiation,
) (Assertion, error) {
	required, allowUV := false, true

	for {
		result, err := channel.assertAttempt(ctx, request, data, credentials, protocol, required, allowUV)

		var failure *Error

		if !errors.As(err, &failure) || failure.Stage != stageCTAP {
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
	case stageCTAP, stageHID, "CBOR decode", "CBOR canonical", "CTAP decode", "info":
		return true
	default:
		return false
	}
}

func (channel *channel) assertAttempt(
	ctx context.Context, request Request, data []byte, credentials []wire.Credential,
	protocol pinProtocolNegotiation, required, allowUV bool,
) (Assertion, error) {
	info, err := channel.info(ctx)
	if err != nil {
		return emptyAssertion(), err
	}

	parameters, err := channel.authParameters(ctx, info, protocol, request.RelyingPartyID, required, allowUV)
	if err != nil {
		return emptyAssertion(), err
	}

	selected, err := channel.selectCredential(ctx, request.RelyingPartyID, credentials, parameters)
	if err != nil {
		return emptyAssertion(), err
	}

	hash := sha256.Sum256(data)
	assertionRequest := wire.AssertionRequest{
		RpId:              request.RelyingPartyID,
		ClientDataHash:    hash[:],
		AllowList:         nil,
		Options:           nil,
		PinUvAuthParam:    nil,
		PinUvAuthProtocol: nil,
	}
	parameters.apply(&assertionRequest)

	if parameters.internalUV {
		verified := true
		assertionRequest.Options = &wire.AssertionOptions{
			Uv: &verified,
			Up: nil,
		}
	}

	if selected != nil {
		assertionRequest.AllowList = &[]wire.Credential{*selected}
	}

	response, err := channel.assertions(ctx, assertionRequest)
	if err != nil {
		return emptyAssertion(), err
	}

	result, err := projectAssertion(request.RelyingPartyID, data, selected, response)
	if err == nil && parameters.internalUV && result.AuthenticatorData[sha256.Size]&byte(wire.UserVerificationFlag) == 0 {
		return emptyAssertion(), keyFailure("UV assertion", ErrProtocol)
	}

	return result, err
}

func clientData(request Request) ([]byte, error) {
	origin, parseErr := url.Parse(request.Origin)
	if parseErr != nil || !validOrigin(origin, request.RelyingPartyID) {
		return nil, keyFailure("origin binding", ErrProtocol)
	}

	suffix, _ := publicsuffix.PublicSuffix(request.RelyingPartyID)
	if suffix == request.RelyingPartyID {
		return nil, keyFailure("relying party suffix", ErrProtocol)
	}

	challenge, err := decodeBase64URL(request.Challenge)
	if err != nil || len(challenge) == 0 {
		return nil, keyFailure("challenge", ErrProtocol)
	}
	// Python's CollectedClientData has this stable field order. Generated models sort fields,
	// so serialize the generated values individually into the prescribed wire sequence.
	value := wire.ClientData{
		Type:        wire.WebauthnGet,
		Challenge:   base64.RawURLEncoding.EncodeToString(challenge),
		Origin:      request.Origin,
		CrossOrigin: false,
	}

	kind, err := json.Marshal(value.Type)
	if err != nil {
		return nil, keyFailure(stageClientData, err)
	}

	encodedChallenge, err := json.Marshal(value.Challenge)
	if err != nil {
		return nil, keyFailure(stageClientData, err)
	}

	encodedOrigin, err := json.Marshal(value.Origin)
	if err != nil {
		return nil, keyFailure(stageClientData, err)
	}

	return fmt.Appendf(nil, string(wire.CollectedClientDataTemplate), kind, encodedChallenge, encodedOrigin), nil
}

func decodeCredentials(ids []string) ([]wire.Credential, error) {
	credentials := make([]wire.Credential, 0, len(ids))

	for _, id := range ids {
		decoded, err := decodeBase64URL(id)
		if err != nil || len(decoded) == 0 {
			return nil, keyFailure("credential", ErrProtocol)
		}

		credentials = append(credentials, wire.Credential{
			Id:   decoded,
			Type: wire.PublicKey,
		})
	}

	return credentials, nil
}

func decodeBase64URL(value string) ([]byte, error) {
	encoding := base64.RawURLEncoding
	if strings.HasSuffix(value, "=") {
		encoding = base64.URLEncoding
	}

	decoded, err := encoding.DecodeString(value)
	if err != nil {
		return nil, keyFailure("base64", err)
	}

	return decoded, nil
}

func (channel *channel) assertions(
	ctx context.Context, request wire.AssertionRequest,
) (*wire.AssertionResponse, error) {
	var response wire.AssertionResponse

	err := channel.call(ctx, wire.GetAssertion, request, &response)
	if err != nil {
		return nil, err
	}

	count := 1
	if response.NumberOfCredentials != nil {
		count = *response.NumberOfCredentials
	}

	if count < 1 || count > maximumAssertions {
		return nil, keyFailure("assertion count", ErrProtocol)
	}

	for range count - 1 {
		var next wire.AssertionResponse

		err := channel.call(ctx, wire.GetNextAssertion, nil, &next)
		if err != nil {
			return nil, err
		}
	}

	return &response, nil
}

//nolint:nilnil // A nil credential represents the protocol's absent optional allow-list (GO-15).
func (channel *channel) selectCredential(
	ctx context.Context, relyingPartyID string, credentials []wire.Credential, parameters authParameters,
) (*wire.Credential, error) {
	if len(credentials) == 0 {
		return nil, nil // An absent allow-list deliberately permits a discoverable credential.
	}

	info, err := channel.info(ctx)
	if err != nil {
		return nil, err
	}

	maximum := 1
	if info.MaxCredentialCountInList != nil && *info.MaxCredentialCountInList > 0 {
		maximum = *info.MaxCredentialCountInList
	}

	filtered := filterCredentials(info, credentials)

	for len(filtered) > 0 {
		count := min(maximum, len(filtered))
		chunk := filtered[:count]
		presence := false
		request := wire.AssertionRequest{
			RpId:           relyingPartyID,
			ClientDataHash: make([]byte, sha256.Size),
			AllowList:      &chunk,
			Options: &wire.AssertionOptions{
				Up: &presence,
				Uv: nil,
			},
			PinUvAuthParam:    nil,
			PinUvAuthProtocol: nil,
		}
		parameters.apply(&request)

		response, err := channel.assertions(ctx, request)
		if err == nil {
			return selectResponseCredential(chunk, response)
		}

		nextMaximum, skipped, retryErr := selectionRetry(err, maximum, count)
		if retryErr != nil {
			return nil, retryErr
		}

		maximum = nextMaximum
		filtered = filtered[skipped:]
	}

	return &wire.Credential{
		Id:   []byte{0},
		Type: wire.PublicKey,
	}, nil
}

func filterCredentials(info *wire.InfoResponse, credentials []wire.Credential) []wire.Credential {
	filtered := make([]wire.Credential, 0, len(credentials))

	for _, credential := range credentials {
		if info.MaxCredentialIdLength == nil || *info.MaxCredentialIdLength == 0 ||
			len(credential.Id) <= *info.MaxCredentialIdLength {
			filtered = append(filtered, credential)
		}
	}

	return filtered
}

func selectResponseCredential(chunk []wire.Credential, response *wire.AssertionResponse) (*wire.Credential, error) {
	if len(chunk) == 1 {
		return &chunk[0], nil
	}

	if response.Credential != nil {
		for _, candidate := range chunk {
			if bytes.Equal(candidate.Id, response.Credential.Id) {
				return response.Credential, nil
			}
		}
	}

	return nil, keyFailure("credential selection", ErrProtocol)
}

func projectAssertion(
	rp string, data []byte, credential *wire.Credential, response *wire.AssertionResponse,
) (Assertion, error) {
	hash := sha256.Sum256([]byte(rp))
	if len(response.AuthData) < int(wire.AuthenticatorHeaderBytes) ||
		!bytes.Equal(response.AuthData[:sha256.Size], hash[:]) ||
		response.AuthData[sha256.Size]&byte(wire.UserPresenceFlag) == 0 || len(response.Signature) == 0 {
		return emptyAssertion(), keyFailure("assertion binding", ErrProtocol)
	}

	selected, err := assertionCredential(credential, response)
	if err != nil {
		return emptyAssertion(), err
	}

	result := Assertion{
		ClientData:        data,
		AuthenticatorData: response.AuthData,
		Signature:         response.Signature,
		CredentialID:      append([]byte(nil), selected.Id...),
		UserHandle:        nil,
	}
	if response.User != nil && response.User.Id != nil {
		result.UserHandle = append([]byte(nil), (*response.User.Id)...)
	}

	return result, nil
}

func assertionCredential(credential *wire.Credential, response *wire.AssertionResponse) (*wire.Credential, error) {
	selected := response.Credential
	if selected == nil {
		selected = credential
	}

	if selected == nil || selected.Type != wire.PublicKey || len(selected.Id) == 0 ||
		(credential != nil && !bytes.Equal(selected.Id, credential.Id)) {
		return nil, keyFailure("assertion credential", ErrProtocol)
	}

	return selected, nil
}

func validOrigin(origin *url.URL, relyingPartyID string) bool {
	if origin.Scheme != "https" || !strings.Contains(relyingPartyID, ".") {
		return false
	}

	return origin.Hostname() == relyingPartyID || strings.HasSuffix(origin.Hostname(), "."+relyingPartyID)
}

func checkedCommand(command wire.CTAPCommand) (byte, error) {
	if command < 0 || command > math.MaxUint8 {
		return 0, keyFailure("CTAP command", ErrProtocol)
	}

	return byte(command), nil
}

func assertionDecoderOptions() cbor.DecOptions {
	return cbor.DecOptions{
		DupMapKey:                cbor.DupMapKeyEnforcedAPF,
		IndefLength:              cbor.IndefLengthForbidden,
		TagsMd:                   cbor.TagsForbidden,
		TimeTag:                  0,
		MaxNestedLevels:          0,
		MaxArrayElements:         0,
		MaxMapPairs:              0,
		IntDec:                   0,
		MapKeyByteString:         0,
		ExtraReturnErrors:        0,
		DefaultMapType:           nil,
		UTF8:                     0,
		FieldNameMatching:        0,
		BigIntDec:                0,
		DefaultByteStringType:    nil,
		ByteStringToString:       0,
		FieldNameByteString:      0,
		UnrecognizedTagToAny:     0,
		TimeTagToAny:             0,
		SimpleValues:             nil,
		NaN:                      0,
		Inf:                      0,
		ByteStringToTime:         0,
		ByteStringExpectedFormat: 0,
		BignumTag:                0,
		BinaryUnmarshaler:        0,
	}
}

func selectionRetry(err error, maximum, count int) (int, int, error) {
	var failure *Error
	if !errors.As(err, &failure) || failure.Stage != stageCTAP {
		return 0, 0, err
	}

	switch failure.Status {
	case int(wire.RequestTooLarge):
		if maximum <= 1 {
			return 0, 0, err
		}

		return maximum - 1, 0, nil
	case int(wire.NoCredentials):
		return maximum, count, nil
	default:
		return 0, 0, err
	}
}
