package bridge_test

import (
	"bytes"
	"github.com/portpowered/go-icloud/pkg/dependencies/bridge"
	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
	pb "github.com/portpowered/go-icloud/pkg/dependencymodels/bridgepb"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

func TestDecodePushDiscardsInvalidUTF8LikeSource(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"flowid":"unit-flow","nextStep":2,"unknown":"ab`)
	payload = append(payload, 0xff)
	payload = append(payload, []byte(`cd"}`)...)

	push, err := bridge.DecodePush(payload)
	if err != nil {
		t.Fatal(err)
	}

	if push.Payload.AdditionalProperties["unknown"] != "abcd" {
		t.Fatalf("Source UTF-8 ignore = %#v", push.Payload.AdditionalProperties)
	}
}

func TestWaitPushDiscardsInvalidUTF8InNamedTopic(t *testing.T) {
	t.Parallel()

	socket := newSessionSocket()
	enqueuePush(t, socket, unitSession, models.ProverShareStep, `,"salt":"`+unitSalt+`"`)
	payload := <-socket.inbound
	message := new(pb.ServerMessage)

	err := proto.Unmarshal(payload, message)
	if err != nil {
		t.Fatal(err)
	}

	message.Push.Topic = append([]byte{0xff}, message.GetPush().GetTopic()...)

	payload, err = proto.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}

	socket.inbound <- payload

	options := bridge.WaitOptions{Timeout: time.Second, Clock: time.Now}

	push, err := bridge.WaitPush(t.Context(), socket, string(models.DefaultTopic), options)
	if err != nil || push.SessionID != unitSession {
		t.Fatalf("Source named topic UTF-8 = %v, %v", push, err)
	}
}

func TestWaitPushTokenPreservesSourceNoncanonicalPadBits(t *testing.T) {
	t.Parallel()

	socket := newSessionSocket()
	message := new(pb.ServerMessage)
	message.Connection = new(pb.ConnectionResponse)
	message.Connection.PushTokenBase64 = []byte("AB==")

	payload, err := proto.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}

	socket.inbound <- payload

	options := bridge.WaitOptions{Timeout: time.Second, Clock: time.Now}

	token, err := bridge.WaitPushToken(t.Context(), socket, options)
	if err != nil || !bytes.Equal(token, []byte{0}) {
		t.Fatalf("Source pad bits = %x, %v", token, err)
	}
}
