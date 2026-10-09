package securitykey

import (
	"encoding/binary"

	wire "github.com/portpowered/go-icloud/pkg/dependencymodels/securitykey"
)

func initialReport(frame wire.InitialFrame) []byte {
	report := make([]byte, int(wire.ReportBytes)+1)
	binary.BigEndian.PutUint32(report[1:], frame.ChannelId)
	report[5] = frame.Command
	binary.BigEndian.PutUint16(report[6:], frame.PayloadLength)
	copy(report[int(wire.InitialHeaderBytes)+1:], frame.Payload)
	return report
}

func continuationReport(frame wire.ContinuationFrame) []byte {
	report := make([]byte, int(wire.ReportBytes)+1)
	binary.BigEndian.PutUint32(report[1:], frame.ChannelId)
	report[5] = frame.Sequence
	copy(report[int(wire.ContinuationHeaderBytes)+1:], frame.Payload)
	return report
}

func initializationResponse(payload []byte) (*wire.InitializationResponse, error) {
	if len(payload) != int(wire.InitResponseBytes) {
		return nil, &Error{Stage: "initialize", Cause: ErrProtocol}
	}
	offset := int(wire.NonceBytes)
	response := &wire.InitializationResponse{Nonce: payload[:offset], AllocatedChannelId: binary.BigEndian.Uint32(payload[offset:]), ProtocolVersion: payload[offset+4], MajorVersion: payload[offset+5], MinorVersion: payload[offset+6], BuildVersion: payload[offset+7], Capabilities: payload[offset+8]}
	return response, nil
}
