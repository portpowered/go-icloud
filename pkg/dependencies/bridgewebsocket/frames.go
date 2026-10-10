package bridgewebsocket

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	pb "github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
	model "github.com/portpowered/go-icloud/pkg/dependencymodels/bridgewebsocket"
)

// A local allocation bound prevents an untrusted frame from exhausting memory.
const maxMessageBytes = 64 * 1024 * 1024

// SendBinary sends a complete masked binary message with caller-owned entropy.
func (c *Conn) SendBinary(ctx context.Context, payload []byte) error {
	return c.sendFrame(ctx, pb.Opcode_OPCODE_BINARY, payload)
}

func (c *Conn) sendFrame(ctx context.Context, opcode pb.Opcode, payload []byte) error {
	err := c.acquire(ctx, c.writeTurn)
	if err != nil {
		return err
	}

	defer func() { <-c.writeTurn }()

	release, err := deadline(ctx, c.socket.SetWriteDeadline)
	if err != nil {
		return err
	}

	defer release()

	mask := make([]byte, int(pb.Framing_FRAMING_MASK_BYTES))

	_, err = io.ReadFull(c.random, mask)
	if err != nil {
		return fmt.Errorf("bridge frame entropy: %w", err)
	}

	encoded := encodeFrame(opcode, payload, mask)

	err = writeAll(c.socket, encoded)
	if err != nil {
		return fmt.Errorf("bridge frame send: %w", contextError(ctx, err))
	}

	return nil
}

func encodeFrame(opcode pb.Opcode, payload, mask []byte) []byte {
	fin := byte(pb.Framing_FRAMING_FIN_MASK)
	// Only schema-owned binary, pong and close opcodes reach this encoder.
	opcodeByte := byte(opcode) //nolint:gosec // Callers supply generated one-byte opcode constants.
	output := []byte{fin | opcodeByte}

	length := len(payload)

	switch {
	case length < int(pb.Framing_FRAMING_SHORT_LIMIT):
		output = append(output, fin|byte(length))
	case length < int(pb.Framing_FRAMING_MEDIUM_LIMIT):
		output = append(output, fin|byte(pb.Framing_FRAMING_SHORT_LIMIT))
		output = binary.BigEndian.AppendUint16(output, uint16(length))
	default:
		output = append(output, fin|byte(pb.Framing_FRAMING_LONG_MARKER))
		output = binary.BigEndian.AppendUint64(output, uint64(length))
	}

	output = append(output, mask...)
	for index, value := range payload {
		output = append(output, value^mask[index%len(mask)])
	}

	return output
}

// ReadMessage assembles text or binary fragments and answers ping frames inline.
func (c *Conn) ReadMessage(ctx context.Context) ([]byte, error) {
	err := c.acquire(ctx, c.readTurn)
	if err != nil {
		return nil, err
	}

	defer func() { <-c.readTurn }()

	release, err := deadline(ctx, c.socket.SetReadDeadline)
	if err != nil {
		return nil, err
	}

	defer release()

	fragments := new(messageFragments)

	for {
		next, err := c.readFrame()
		if err != nil {
			return nil, fmt.Errorf("bridge frame read: %w", contextError(ctx, err))
		}

		handled, err := c.handleControl(ctx, next)
		if err != nil {
			return nil, err
		}

		if handled {
			continue
		}

		payload, finished, err := fragments.accept(next)
		if err != nil {
			return nil, err
		}

		if finished {
			return payload, nil
		}
	}
}

type messageFragments struct {
	payload []byte
	opcode  pb.Opcode
}

func (m *messageFragments) accept(next model.Frame) ([]byte, bool, error) {
	if next.Opcode != pb.Opcode_OPCODE_CONTINUATION {
		m.opcode = next.Opcode
	}

	if len(m.payload) > maxMessageBytes-len(next.Payload) {
		return nil, false, ErrProtocol
	}

	m.payload = append(m.payload, next.Payload...)

	if !next.Finished {
		return nil, false, nil
	}

	if m.opcode != pb.Opcode_OPCODE_BINARY && m.opcode != pb.Opcode_OPCODE_TEXT {
		return nil, false, ErrProtocol
	}

	return m.payload, true, nil
}

func (c *Conn) handleControl(ctx context.Context, next model.Frame) (bool, error) {
	switch next.Opcode {
	case pb.Opcode_OPCODE_CLOSE:
		return true, ErrClosed
	case pb.Opcode_OPCODE_PING:
		return true, c.sendFrame(ctx, pb.Opcode_OPCODE_PONG, next.Payload)
	case pb.Opcode_OPCODE_PONG:
		return true, nil
	case pb.Opcode_OPCODE_CONTINUATION, pb.Opcode_OPCODE_TEXT, pb.Opcode_OPCODE_BINARY:
		return false, nil
	default:
		return false, nil
	}
}

func (c *Conn) readFrame() (model.Frame, error) {
	result := new(model.Frame)

	header := make([]byte, int(model.MediumLengthBytes))

	_, err := io.ReadFull(c.reader, header)
	if err != nil {
		return *result, fmt.Errorf("bridge frame header: %w", err)
	}

	fin := byte(pb.Framing_FRAMING_FIN_MASK)
	result.Opcode = pb.Opcode(header[model.FrameOpcodePosition] & byte(model.FrameOpcodeMask))
	result.Finished = header[model.FrameOpcodePosition]&fin != 0

	length, err := c.readLength(header[model.FrameLengthPosition] & byte(pb.Framing_FRAMING_LONG_MARKER))
	if err != nil {
		return *result, err
	}

	if length > maxMessageBytes {
		return *result, ErrProtocol
	}

	mask := []byte{}
	if header[model.FrameLengthPosition]&fin != 0 {
		mask = make([]byte, int(pb.Framing_FRAMING_MASK_BYTES))

		_, err = io.ReadFull(c.reader, mask)
		if err != nil {
			return *result, fmt.Errorf("bridge frame mask: %w", err)
		}
	}

	result.Payload = make([]byte, int(length))

	_, err = io.ReadFull(c.reader, result.Payload)
	if err != nil {
		return *result, fmt.Errorf("bridge frame payload: %w", err)
	}

	if len(mask) > 0 {
		for index := range result.Payload {
			result.Payload[index] ^= mask[index%len(mask)]
		}
	}

	return *result, nil
}

func (c *Conn) readLength(length byte) (uint64, error) {
	var size int

	switch length {
	case byte(pb.Framing_FRAMING_SHORT_LIMIT):
		size = int(model.MediumLengthBytes)
	case byte(pb.Framing_FRAMING_LONG_MARKER):
		size = int(model.LongLengthBytes)
	default:
		return uint64(length), nil
	}

	encoded := make([]byte, size)

	_, err := io.ReadFull(c.reader, encoded)
	if err != nil {
		return 0, fmt.Errorf("bridge frame length: %w", err)
	}

	if size == int(model.MediumLengthBytes) {
		return uint64(binary.BigEndian.Uint16(encoded)), nil
	}

	return binary.BigEndian.Uint64(encoded), nil
}
