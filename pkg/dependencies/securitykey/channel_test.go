//nolint:testpackage // White-box protocol and injected HID lifecycle tests require unexported seams (GO-15).
package securitykey

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"testing"
	"time"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

func TestRejectOversizedMessage(t *testing.T) {
	t.Parallel()

	channel := &channel{
		connection:      nil,
		id:              0,
		ctap2:           false,
		wait:            nil,
		entropy:         nil,
		maxMessageBytes: 0,
	}

	err := channel.write(t.Context(), byte(wire.CBORCommand), make([]byte, int(wire.MaxPayloadBytes)+1))
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("oversized payload accepted: %v", err)
	}
}

func TestRejectContinuationSequence(t *testing.T) {
	t.Parallel()

	initial := make([]byte, int(wire.ReportBytes))
	initial[4] = byte(wire.CBORCommand) | byte(wire.InitialFlag)
	binary.BigEndian.PutUint16(initial[5:], 100)

	continuation := make([]byte, int(wire.ReportBytes))
	continuation[4] = 1
	connection := &fakeConnection{
		t: t,
		reads: [][]byte{
			initial,
			continuation,
		},
		writes:            nil,
		closed:            0,
		cancelOnRead:      false,
		cancelAfterReads:  0,
		readCount:         0,
		readFailure:       nil,
		failureAfterReads: 0,
		closeFailure:      nil,
	}
	channel := &channel{
		connection:      connection,
		id:              0,
		ctap2:           false,
		wait:            nil,
		entropy:         nil,
		maxMessageBytes: 0,
	}

	_, err := channel.read(t.Context(), byte(wire.CBORCommand))
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("out of order continuation accepted: %v", err)
	}
}

func TestKeepalive(t *testing.T) {
	t.Parallel()

	keepalive := make([]byte, int(wire.ReportBytes))
	keepalive[4] = byte(wire.KeepaliveCommand) | byte(wire.InitialFlag)
	keepalive[6] = 1
	keepalive[7] = 2
	response := make([]byte, int(wire.ReportBytes))
	response[4] = byte(wire.CBORCommand) | byte(wire.InitialFlag)
	response[6] = 1
	response[7] = 99
	connection := &fakeConnection{
		t: t,
		reads: [][]byte{
			keepalive,
			response,
		},
		writes:            nil,
		closed:            0,
		cancelOnRead:      false,
		cancelAfterReads:  0,
		readCount:         0,
		readFailure:       nil,
		failureAfterReads: 0,
		closeFailure:      nil,
	}
	channel := &channel{
		connection:      connection,
		id:              0,
		ctap2:           false,
		wait:            nil,
		entropy:         nil,
		maxMessageBytes: 0,
	}

	result, err := channel.read(t.Context(), byte(wire.CBORCommand))
	if err != nil || !bytes.Equal(result, []byte{
		99,
	}) || len(connection.reads) != 0 {
		t.Fatalf("keepalive handling failed: %x, %v", result, err)
	}
}

func TestRejectWrongChannelAndKeepaliveStatus(t *testing.T) {
	t.Parallel()

	for _, wrongChannel := range []bool{true, false} {
		packet := make([]byte, int(wire.ReportBytes))
		packet[4] = byte(wire.KeepaliveCommand) | byte(wire.InitialFlag)
		packet[6] = 1

		packet[7] = 3

		if wrongChannel {
			packet[0] = 1
			packet[7] = byte(wire.Processing)
		}

		connection := &fakeConnection{
			t: t,
			reads: [][]byte{
				packet,
			},
			writes:            nil,
			closed:            0,
			cancelOnRead:      false,
			cancelAfterReads:  0,
			readCount:         0,
			readFailure:       nil,
			failureAfterReads: 0,
			closeFailure:      nil,
		}
		channel := &channel{
			connection:      connection,
			id:              0,
			ctap2:           false,
			wait:            nil,
			entropy:         nil,
			maxMessageBytes: 0,
		}

		_, err := channel.read(t.Context(), byte(wire.CBORCommand))
		if !errors.Is(err, ErrProtocol) {
			t.Fatalf("invalid channel/keepalive accepted: %v", err)
		}
	}
}

func TestContentionWaitCancellation(t *testing.T) {
	t.Parallel()
	fixture := readTranscript(t)
	write := decodeHex(t, fixture.Steps[1].Writes[0])
	response := make([]byte, int(wire.ReportBytes))
	copy(response, []byte{
		1,
		2,
		3,
		4,
	})

	response[4] = byte(wire.ErrorCommand) | byte(wire.InitialFlag)
	response[6] = 1
	response[7] = byte(wire.ChannelBusy)
	connection := &fakeConnection{
		t: t,
		writes: [][]byte{
			write,
		},
		reads: [][]byte{
			response,
		},
		closed:            0,
		cancelOnRead:      false,
		cancelAfterReads:  0,
		readCount:         0,
		readFailure:       nil,
		failureAfterReads: 0,
		closeFailure:      nil,
	}
	waits := 0
	channel := &channel{
		connection: connection,
		id:         0x01020304,
		wait: func(_ context.Context, delay time.Duration) error {
			waits++

			if delay != busyDelay {
				t.Fatal("incorrect contention backoff")
			}

			return context.Canceled
		},
		ctap2:           false,
		entropy:         nil,
		maxMessageBytes: 0,
	}

	_, err := channel.exchange(t.Context(), byte(wire.CBORCommand), []byte{
		byte(wire.GetInfo),
	})
	if !errors.Is(err, context.Canceled) || waits != 1 || len(connection.writes) != 0 {
		t.Fatalf("contention cancellation failed: %v", err)
	}
}

func TestProductionBoundaryHonorsCanceledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := (HIDBackend{
		enumerate: nil,
		open:      nil,
	}).Devices(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("discovery cancellation lost: %v", err)
	}

	_, err = (HIDBackend{
		enumerate: nil,
		open:      nil,
	}).Open(ctx, "unused")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("open cancellation lost: %v", err)
	}
}

func TestRejectInvalidCBOR(t *testing.T) {
	t.Parallel()

	cases := map[string][]byte{
		"missing status":       {},
		"noncanonical integer": {0, 0xa1, 0x18, 0x01, 0x00},
		"duplicate key":        {0, 0xa2, 1, 0, 1, 0},
		"indefinite map":       {0, 0xbf, 0xff},
		"trailing value":       {0, 0xa0, 0xa0},
		"error status":         {byte(wire.NoCredentials)},
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := readTranscript(t)
			response := make([]byte, int(wire.ReportBytes))
			copy(response, []byte{
				1,
				2,
				3,
				4,
			})

			response[4] = byte(wire.CBORCommand) | byte(wire.InitialFlag)

			payloadLength := len(payload)
			if payloadLength > math.MaxUint16 || payloadLength > int(wire.MaxPayloadBytes) {
				t.Fatal("fixture payload exceeds HID limit")
			}

			binary.BigEndian.PutUint16(response[5:], uint16(payloadLength))
			copy(response[7:], payload)
			connection := &fakeConnection{
				t: t,
				writes: [][]byte{
					decodeHex(t, fixture.Steps[1].Writes[0]),
				},
				reads: [][]byte{
					response,
				},
				closed:            0,
				cancelOnRead:      false,
				cancelAfterReads:  0,
				readCount:         0,
				readFailure:       nil,
				failureAfterReads: 0,
				closeFailure:      nil,
			}
			channel := &channel{
				connection:      connection,
				id:              0x01020304,
				ctap2:           false,
				wait:            nil,
				entropy:         nil,
				maxMessageBytes: 0,
			}

			var result wire.InfoResponse

			err := channel.call(t.Context(), wire.GetInfo, nil, &result)
			if err == nil {
				t.Fatal("invalid CBOR accepted")
			}
		})
	}
}

func TestU2FHardwareFailurePropagates(t *testing.T) {
	t.Parallel()
	fixture := readTranscriptFile(t, "python-u2f-synthetic.json")
	connection := transcriptConnection(t, transcript{
		Steps:             fixture.Steps[:2],
		ClientData:        "",
		AuthenticatorData: "",
		CredentialID:      "",
		CredentialIDs:     nil,
		Signature:         "",
		Error:             "",
	})
	failure := errSyntheticRead
	connection.readFailure = failure
	connection.failureAfterReads = 1

	provider, err := New(fakeBackend{
		connection: connection,
		discovery:  nil,
	}, bytes.NewReader([]byte{
		0,
		1,
		2,
		3,
		4,
		5,
		6,
		7,
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.Assert(t.Context(), Request{
		DeviceID:       syntheticDeviceID,
		RelyingPartyID: syntheticRelyingPartyID,
		Origin:         syntheticOrigin,
		Challenge:      syntheticChallenge,
		CredentialIDs: []string{
			syntheticCredentialID,
			"u8zd",
		},
	})
	if !errors.Is(err, failure) || connection.closed != 1 || len(connection.writes) != 0 {
		t.Fatalf("hardware error converted into credential refusal: %v", err)
	}
}

func TestU2FMalformedAPDUPropagates(t *testing.T) {
	t.Parallel()
	fixture := readTranscriptFile(t, "python-u2f-synthetic.json")
	connection := transcriptConnection(t, transcript{
		Steps:             fixture.Steps[:2],
		ClientData:        "",
		AuthenticatorData: "",
		CredentialID:      "",
		CredentialIDs:     nil,
		Signature:         "",
		Error:             "",
	})
	connection.reads[1][6] = 1
	connection.reads[1][7] = 0

	provider, err := New(fakeBackend{
		connection: connection,
		discovery:  nil,
	}, bytes.NewReader([]byte{
		0,
		1,
		2,
		3,
		4,
		5,
		6,
		7,
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.Assert(t.Context(), Request{
		DeviceID:       syntheticDeviceID,
		RelyingPartyID: syntheticRelyingPartyID,
		Origin:         syntheticOrigin,
		Challenge:      syntheticChallenge,
		CredentialIDs: []string{
			syntheticCredentialID,
			"u8zd",
		},
	})

	var failure *Error

	if !errors.As(err, &failure) || failure.Stage != "APDU framing" ||
		connection.closed != 1 || len(connection.writes) != 0 {
		t.Fatalf("malformed APDU converted into credential refusal: %v", err)
	}
}

func TestEntropyAndCloseCausesPreserved(t *testing.T) {
	t.Parallel()

	closeFailure := errSyntheticClose
	connection := &fakeConnection{
		t:                 t,
		closeFailure:      closeFailure,
		writes:            nil,
		reads:             nil,
		closed:            0,
		cancelOnRead:      false,
		cancelAfterReads:  0,
		readCount:         0,
		readFailure:       nil,
		failureAfterReads: 0,
	}

	provider, err := New(fakeBackend{
		connection: connection,
		discovery:  nil,
	}, bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.Assert(t.Context(), Request{
		DeviceID:       syntheticDeviceID,
		RelyingPartyID: syntheticRelyingPartyID,
		Origin:         syntheticOrigin,
		Challenge:      syntheticChallenge,
		CredentialIDs:  nil,
	})
	if !errors.Is(err, closeFailure) || connection.closed != 1 {
		t.Fatalf("close failure lost: %v", err)
	}
}

func TestRejectPublicSuffixDelegation(t *testing.T) {
	t.Parallel()

	for _, relyingPartyID := range []string{
		"com",
		"co.uk",
		"github.io",
	} {
		_, err := clientData(Request{
			Origin:         "https://example." + relyingPartyID,
			RelyingPartyID: relyingPartyID,
			Challenge:      syntheticChallenge,
			DeviceID:       "",
			CredentialIDs:  nil,
		})
		if !errors.Is(err, ErrProtocol) {
			t.Fatalf("public suffix accepted: %s, %v", relyingPartyID, err)
		}
	}
}
