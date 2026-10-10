package reminderstext_test

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/internal/reminderstext"
)

type encodingEntity struct {
	Value string `json:"value"`
}

type encodingCase struct {
	Name   string          `json:"name"`
	Input  encodingEntity  `json:"input"`
	Result *encodingEntity `json:"result"`
}

type encodingCorpus struct {
	Cases []encodingCase `json:"cases"`
}

func TestEncodeMatchesReferenceProtobuf(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../tests/replay/fixtures/synthetic/binary/reminders-text.json")
	if err != nil {
		t.Fatal(err)
	}

	var corpus encodingCorpus

	err = json.Unmarshal(data, &corpus)
	if err != nil {
		t.Fatal(err)
	}

	count := 0

	for _, item := range corpus.Cases {
		if item.Result == nil || !strings.HasPrefix(item.Name, "current-") || !strings.HasSuffix(item.Name, "-raw") {
			continue
		}

		count++

		t.Run(item.Name, func(t *testing.T) {
			t.Parallel()
			checkEncodedReference(t, item)
		})
	}

	if count == 0 {
		t.Fatal("no Source encoder evidence was consumed")
	}
}

func checkEncodedReference(t *testing.T, item encodingCase) {
	t.Helper()

	text, err := base64.StdEncoding.DecodeString(item.Result.Value)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := reminderstext.Encode(string(text))
	if err != nil {
		t.Fatal(err)
	}

	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}

	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}

	actual, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	err = reader.Close()
	if err != nil {
		t.Fatal(err)
	}

	expected, err := base64.StdEncoding.DecodeString(item.Input.Value)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(actual, expected) {
		t.Fatal("complete versioned protobuf differs from Source")
	}
}
