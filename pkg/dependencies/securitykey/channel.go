package securitykey

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"time"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

const busyDelay = 100 * time.Millisecond
const cancelTimeout = time.Second

type channel struct {
	connection Connection
	id         uint32
	ctap2      bool
	wait       Waiter
	entropy    io.Reader
}

func (channel *channel) initialize(ctx context.Context, nonce []byte) error {
	response, err := channel.exchange(ctx, byte(wire.InitCommand), nonce)
	if err != nil {
		return err
	}
	if len(response) != int(wire.InitResponseBytes) || !bytes.Equal(response[:len(nonce)], nonce) {
		return &Error{Stage: "initialize", Cause: ErrProtocol}
	}
	id := binary.BigEndian.Uint32(response[int(wire.NonceBytes):])
	if id == 0 || id == uint32(wire.BroadcastChannel) {
		return &Error{Stage: "channel", Cause: ErrProtocol}
	}
	channel.id = id
	channel.ctap2 = response[len(response)-1]&byte(wire.CBORCapability) != 0
	return nil
}

func (channel *channel) exchange(ctx context.Context, command byte, payload []byte) ([]byte, error) {
	for {
		if err := channel.write(ctx, command, payload); err != nil {
			return nil, &Error{Stage: "write", Cause: err}
		}
		response, err := channel.read(ctx, command)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			channel.cancel()
			return nil, err
		}
		var failure *Error
		if !errors.As(err, &failure) || failure.Stage != "HID" || failure.Status != int(wire.ChannelBusy) {
			return response, err
		}
		if err := channel.wait(ctx, busyDelay); err != nil {
			return nil, &Error{Stage: "busy", Cause: err}
		}
	}
}

func (channel *channel) cancel() {
	ctx, cancel := context.WithTimeout(context.Background(), cancelTimeout)
	defer cancel()
	// Cancellation is best effort; the original context failure remains authoritative.
	_ = channel.write(ctx, byte(wire.CancelCommand), nil)
}

func (channel *channel) write(ctx context.Context, command byte, payload []byte) error {
	if len(payload) > int(wire.MaxPayloadBytes) {
		return ErrProtocol
	}
	header := int(wire.InitialHeaderBytes)
	remaining := payload
	sequence := byte(0)
	for {
		packet := make([]byte, int(wire.ReportBytes)+1)
		binary.BigEndian.PutUint32(packet[1:], channel.id)
		if header == int(wire.InitialHeaderBytes) {
			packet[5] = command | byte(wire.InitialFlag)
			binary.BigEndian.PutUint16(packet[6:], uint16(len(payload)))
		} else {
			packet[5] = sequence
			sequence++
		}
		copied := copy(packet[header+1:], remaining)
		count, err := channel.connection.Write(ctx, packet)
		if err != nil {
			return err
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
		packet := make([]byte, int(wire.ReportBytes))
		count, err := channel.connection.Read(ctx, packet)
		if err != nil {
			return nil, &Error{Stage: "read", Cause: err}
		}
		if count != len(packet) {
			return nil, &Error{Stage: "report", Cause: ErrProtocol}
		}
		if binary.BigEndian.Uint32(packet) != channel.id {
			continue
		}
		if packet[4]&byte(wire.InitialFlag) != 0 {
			responseCommand := packet[4] &^ byte(wire.InitialFlag)
			size := int(binary.BigEndian.Uint16(packet[5:]))
			if responseCommand == byte(wire.KeepaliveCommand) && target == -1 && size == 1 {
				continue
			}
			if responseCommand == byte(wire.ErrorCommand) && size == 1 {
				return nil, &Error{Stage: "HID", Status: int(packet[7]), Cause: ErrProtocol}
			}
			if responseCommand != command || target != -1 || size > int(wire.MaxPayloadBytes) {
				return nil, &Error{Stage: "initial frame", Cause: ErrProtocol}
			}
			target = size
			payload = append(payload, packet[int(wire.InitialHeaderBytes):]...)
		} else {
			if target == -1 || packet[4] != sequence {
				return nil, &Error{Stage: "continuation", Cause: ErrProtocol}
			}
			sequence++
			payload = append(payload, packet[int(wire.ContinuationHeaderBytes):]...)
		}
		if len(payload) >= target {
			return payload[:target], nil
		}
	}
}
