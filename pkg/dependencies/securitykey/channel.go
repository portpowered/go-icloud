package securitykey

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"time"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

const busyDelay = 100 * time.Millisecond
const cancelTimeout = time.Second

type channel struct {
	connection      Connection
	id              uint32
	ctap2           bool
	wait            Waiter
	entropy         io.Reader
	maxMessageBytes int
}

func (channel *channel) initialize(ctx context.Context, nonce []byte) error {
	response, err := channel.exchange(ctx, byte(wire.InitCommand), nonce)
	if err != nil {
		return err
	}

	initialized, err := initializationResponse(response)
	if err != nil {
		return err
	}

	if !bytes.Equal(initialized.Nonce, nonce) {
		return keyFailure("initialize", ErrProtocol)
	}

	channelID := initialized.AllocatedChannelId
	if channelID == 0 || channelID == uint32(wire.BroadcastChannel) {
		return keyFailure("channel", ErrProtocol)
	}

	channel.id = channelID
	channel.ctap2 = initialized.Capabilities&byte(wire.CBORCapability) != 0

	return nil
}

func (channel *channel) exchange(ctx context.Context, command byte, payload []byte) ([]byte, error) {
	for {
		err := channel.write(ctx, command, payload)
		if err != nil {
			return nil, keyFailure("write", err)
		}

		response, err := channel.read(ctx, command)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			channel.cancel(ctx)

			return nil, err
		}

		var failure *Error
		if !errors.As(err, &failure) || failure.Stage != stageHID || failure.Status != int(wire.ChannelBusy) {
			return response, err
		}

		err = channel.wait(ctx, busyDelay)
		if err != nil {
			return nil, keyFailure("busy", err)
		}
	}
}

func (channel *channel) cancel(parent context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), cancelTimeout)
	defer cancel()
	// Cancellation is best effort; the original context failure remains authoritative.
	_ = channel.write(ctx, byte(wire.CancelCommand), nil)
}

func (channel *channel) write(ctx context.Context, command byte, payload []byte) error {
	payloadLength := len(payload)
	if payloadLength > math.MaxUint16 || payloadLength > int(wire.MaxPayloadBytes) {
		return ErrProtocol
	}

	header := int(wire.InitialHeaderBytes)
	remaining := payload
	sequence := byte(0)

	for {
		copied := min(int(wire.ReportBytes)-header, len(remaining))

		var packet []byte

		if header == int(wire.InitialHeaderBytes) {
			packet = initialReport(wire.InitialFrame{
				ChannelId:     channel.id,
				Command:       command | byte(wire.InitialFlag),
				PayloadLength: uint16(payloadLength),
				Payload:       remaining[:copied],
			})
		} else {
			packet = continuationReport(wire.ContinuationFrame{
				ChannelId: channel.id,
				Sequence:  sequence,
				Payload:   remaining[:copied],
			})
			sequence++
		}

		count, err := channel.connection.Write(ctx, packet)
		if err != nil {
			return keyFailure("write", err)
		}

		if count != len(packet) {
			return ErrProtocol
		}

		remaining = remaining[copied:]
		if len(remaining) == 0 {
			return nil
		}

		header = int(wire.ContinuationHeaderBytes)
	}
}

func (channel *channel) read(ctx context.Context, command byte) ([]byte, error) {
	var payload []byte

	target := -1
	sequence := byte(0)

	for {
		packet, err := channel.readReport(ctx)
		if err != nil {
			return nil, err
		}

		if packet[4]&byte(wire.InitialFlag) != 0 {
			size, skipped, err := initialResponse(packet, command, target)
			if err != nil {
				return nil, err
			}

			if skipped {
				continue
			}

			target = size

			payload = append(payload, packet[int(wire.InitialHeaderBytes):]...)
		} else {
			if target == -1 || packet[4] != sequence {
				return nil, keyFailure("continuation", ErrProtocol)
			}

			sequence++

			payload = append(payload, packet[int(wire.ContinuationHeaderBytes):]...)
		}

		if len(payload) >= target {
			return payload[:target], nil
		}
	}
}

func initialResponse(packet []byte, command byte, target int) (int, bool, error) {
	responseCommand := packet[4] &^ byte(wire.InitialFlag)

	size := int(binary.BigEndian.Uint16(packet[5:]))

	if responseCommand == byte(wire.KeepaliveCommand) && target == -1 && size == 1 {
		if !wire.KeepaliveStatus(packet[7]).Valid() {
			return 0, false, keyFailure("keepalive status", ErrProtocol)
		}

		return 0, true, nil
	}

	if responseCommand == byte(wire.ErrorCommand) && size == 1 {
		return 0, false, &Error{
			Stage:  stageHID,
			Status: int(packet[7]),
			Cause:  ErrProtocol,
		}
	}

	if responseCommand != command || target != -1 || size > int(wire.MaxPayloadBytes) {
		return 0, false, keyFailure("initial frame", ErrProtocol)
	}

	return size, false, nil
}

func (channel *channel) readReport(ctx context.Context) ([]byte, error) {
	packet := make([]byte, int(wire.ReportBytes))
	count, err := channel.connection.Read(ctx, packet)
	if err != nil {
		return nil, keyFailure("read", err)
	}

	if count != len(packet) {
		return nil, keyFailure("report", ErrProtocol)
	}

	if binary.BigEndian.Uint32(packet) != channel.id {
		return nil, keyFailure("response channel", ErrProtocol)
	}

	return packet, nil
}
