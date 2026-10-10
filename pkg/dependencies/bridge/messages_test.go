package bridge_test

import (
	"testing"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// Synthetic duplicate fields exercise the first-value selections in the pinned
// Python _decode_fields consumers; repeated subscription apps remain ordered.
func TestServerDuplicateFields(t *testing.T) {
	t.Parallel()

	first := new(bridgepb.ServerMessage)
	first.Connection = new(bridgepb.ConnectionResponse)
	first.Connection.PushTokenBase64 = []byte("first")
	first.Connection.Status = proto.Uint64(0)
	first.Push = &bridgepb.PushMessage{Topic: []byte("topic"), MessageId: proto.Uint64(1), Payload: []byte("payload")}
	first.Subscription = new(bridgepb.SubscriptionResponse)
	first.Subscription.Payload = new(bridgepb.SubscriptionPayload)
	first.Subscription.Payload.Apps = []*bridgepb.AppSubscriptionResponse{{Topic: []byte("one")}, {Topic: []byte("two")}}
	second := new(bridgepb.ServerMessage)
	second.Connection = new(bridgepb.ConnectionResponse)
	second.Connection.PushTokenBase64 = []byte("second")
	second.Connection.Status = proto.Uint64(2)
	second.Push = &bridgepb.PushMessage{Topic: []byte("other"), MessageId: proto.Uint64(2),
		Payload: []byte("other payload")}
	encoded := duplicateWire(t, first, second)

	decoded, err := bridge.DecodeServerMessage(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if !proto.Equal(decoded, first) {
		t.Fatalf("first singular fields: %v", decoded)
	}

	connection := duplicateWire(t, first.GetConnection(), second.GetConnection())
	push := duplicateWire(t, first.GetPush(), second.GetPush())
	nested := new(bridgepb.ServerMessage)
	nested.ProtoReflect().SetUnknown(append(lengthDelimited(1, connection), lengthDelimited(2, push)...))

	decoded, err = bridge.DecodeServerMessage(duplicateWire(t, nested))
	if err != nil {
		t.Fatal(err)
	}

	if !proto.Equal(decoded.GetConnection(), first.GetConnection()) || !proto.Equal(decoded.GetPush(), first.GetPush()) {
		t.Fatalf("nested first singular fields: %v", decoded)
	}
}

func duplicateWire(t *testing.T, messages ...proto.Message) []byte {
	t.Helper()

	var wire []byte

	for _, message := range messages {
		encoded, err := proto.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}

		wire = append(wire, encoded...)
	}

	return wire
}

func lengthDelimited(number protowire.Number, value []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(nil, number, protowire.BytesType), value)
}
