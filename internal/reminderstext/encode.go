package reminderstext

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"unicode/utf16"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/internal/reminderstext/pb"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/cloudkit"
	"google.golang.org/protobuf/proto"
)

// Encode produces a versioned topotext document, counting offsets in UTF-16 units.
// DEFLATE bytes may differ across encoders; the complete decoded protobuf is stable.
func Encode(text string) (string, error) {
	length := len(utf16.Encode([]rune(text)))
	if uint64(length) > math.MaxUint32 {
		return "", fmt.Errorf("encode reminder text length: %w", errDocument)
	}

	value := new(pb.String)
	value.String_ = proto.String(text)

	value.Substring = []*pb.Substring{substring(uint32(cloudkit.ReminderTextSentinelReplicaValue), 0, 0,
		[]uint32{uint32(cloudkit.ReminderTextContentReplicaValue)})}

	if length != 0 {
		value.Substring = append(value.Substring, substring(uint32(cloudkit.ReminderTextContentReplicaValue),
			0, uint32(length), []uint32{uint32(cloudkit.ReminderTextContentChildValue)}))
	}

	value.Substring = append(value.Substring, substring(uint32(cloudkit.ReminderTextSentinelReplicaValue),
		uint32(cloudkit.ReminderTextTerminalClockValue), 0, nil))

	stamp, err := textTimestamp(uint32(length))
	if err != nil {
		return "", err
	}

	value.Timestamp = stamp

	if length != 0 {
		run := new(pb.AttributeRun)
		run.Length = proto.Uint32(uint32(length))
		value.AttributeRun = []*pb.AttributeRun{run}
	}

	data, err := proto.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode reminder string: %w", err)
	}

	version := new(pb.Version)
	version.SerializationVersion = proto.Uint32(uint32(cloudkit.ReminderTextSerializationVersionValue))
	version.MinimumSupportedVersion = proto.Uint32(uint32(cloudkit.ReminderTextSerializationVersionValue))
	version.Data = data
	document := new(pb.Document)
	document.SerializationVersion = proto.Uint32(uint32(cloudkit.ReminderTextSerializationVersionValue))
	document.Version = []*pb.Version{version}

	data, err = proto.Marshal(document)
	if err != nil {
		return "", fmt.Errorf("encode reminder envelope: %w", err)
	}

	return compressDocument(data)
}

func substring(replica, clock, length uint32, children []uint32) *pb.Substring {
	identity := new(pb.CharID)
	identity.ReplicaID = proto.Uint32(replica)
	identity.Clock = proto.Uint32(clock)
	stamp := new(pb.CharID)
	stamp.ReplicaID = proto.Uint32(replica)
	stamp.Clock = proto.Uint32(clock)
	part := new(pb.Substring)
	part.CharID = identity
	part.Timestamp = stamp
	part.Length = proto.Uint32(length)
	part.Child = children

	return part
}

func textTimestamp(length uint32) (*pb.VectorTimestamp, error) {
	identity, err := hex.DecodeString(protocol.RemindersReminderTextReplicaIdentityValue)
	if err != nil {
		return nil, fmt.Errorf("decode reminder replica identity: %w", err)
	}

	content := new(pb.VectorTimestamp_Clock_ReplicaClock)
	content.Clock = proto.Uint32(length)
	sentinel := new(pb.VectorTimestamp_Clock_ReplicaClock)
	sentinel.Clock = proto.Uint32(uint32(cloudkit.ReminderTextContentReplicaValue))
	clock := new(pb.VectorTimestamp_Clock)
	clock.ReplicaUUID = identity
	clock.ReplicaClock = []*pb.VectorTimestamp_Clock_ReplicaClock{content, sentinel}
	stamp := new(pb.VectorTimestamp)
	stamp.Clock = []*pb.VectorTimestamp_Clock{clock}

	return stamp, nil
}

func compressDocument(data []byte) (string, error) {
	var output bytes.Buffer

	writer := zlib.NewWriter(&output)

	_, err := writer.Write(data)
	if err != nil {
		return "", fmt.Errorf("compress reminder document: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return "", fmt.Errorf("finish reminder document: %w", err)
	}

	return base64.StdEncoding.EncodeToString(output.Bytes()), nil
}
