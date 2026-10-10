//nolint:testpackage // GO-15: asserts private PIN negotiation and token-exchange state before HID traffic.
package securitykey

import (
	"errors"
	"testing"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

func TestMissingPINProtocolCannotStartTokenExchange(t *testing.T) {
	t.Parallel()

	options := map[string]bool{string(wire.Uv): true, string(wire.PinUvAuthToken): true}
	info := &wire.InfoResponse{
		Options: &options, Aaguid: nil, MaxCredentialCountInList: nil,
		MaxCredentialIdLength: nil, MaxMsgSize: nil, PinUvAuthProtocols: nil, Versions: nil,
	}
	transport := &channel{
		connection: nil, id: 0, ctap2: false, wait: nil, entropy: nil, maxMessageBytes: 0,
	}

	for _, protocols := range [][]int{nil, {}, {0}, {99}} {
		info.PinUvAuthProtocols = &protocols

		parameters, err := transport.authParameters(t.Context(), info, negotiatedProtocol(info),
			syntheticRelyingPartyID, true, true)
		if !errors.Is(err, ErrUnsupported) || len(parameters.token) != 0 || parameters.internalUV {
			t.Fatalf("unsupported negotiation started token exchange: protocols=%v parameters=%+v err=%v",
				protocols, parameters, err)
		}
	}
}

func TestPINNegotiationPrefersSupportedVersionTwo(t *testing.T) {
	t.Parallel()

	protocols := []int{int(wire.PINProtocolV1), int(wire.PINProtocolV2)}
	info := &wire.InfoResponse{
		Options: nil, Aaguid: nil, MaxCredentialCountInList: nil,
		MaxCredentialIdLength: nil, MaxMsgSize: nil, PinUvAuthProtocols: &protocols, Versions: nil,
	}

	selection := negotiatedProtocol(info)
	if !selection.supported || selection.protocol != wire.PINProtocolV2 {
		t.Fatalf("PIN protocol preference changed: %+v", selection)
	}
}
