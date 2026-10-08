package replay_test

import (
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/portpowered/go-icloud/internal/reminderstext"
)

type reminderTextEntity struct {
	Encoding string `json:"encoding"`
	Value    string `json:"value"`
}

type reminderTextFailure struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type reminderTextCase struct {
	Name   string               `json:"name"`
	Input  reminderTextEntity   `json:"input"`
	Result *reminderTextEntity  `json:"result,omitempty"`
	Error  *reminderTextFailure `json:"error,omitempty"`
}

type reminderTextSource struct {
	URL    string `json:"url"`
	Commit string `json:"commit"`
}

type reminderTextCorpus struct {
	Source   reminderTextSource `json:"source"`
	Evidence string             `json:"evidence"`
	Format   string             `json:"format"`
	Cases    []reminderTextCase `json:"cases"`
}

func TestRemindersTextReferenceCorpus(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("fixtures/synthetic/binary/reminders-text.json")
	if err != nil {
		t.Fatal(err)
	}

	var corpus reminderTextCorpus

	err = json.Unmarshal(data, &corpus)
	if err != nil {
		t.Fatal(err)
	}

	if corpus.Format != "portos.reminders-text.v1" || len(corpus.Cases) != 52 {
		t.Fatalf("unexpected Reminders text corpus: %s, %d", corpus.Format, len(corpus.Cases))
	}

	checkReminderTextProvenance(t, corpus)

	for _, item := range corpus.Cases {
		t.Run(item.Name, func(t *testing.T) {
			t.Parallel()
			checkReminderText(t, item)
		})
	}
}

func checkReminderText(t *testing.T, item reminderTextCase) {
	t.Helper()

	if item.Input.Encoding != testBase64Encoding {
		t.Fatalf("unsupported document encoding: %s", item.Input.Encoding)
	}

	data, err := base64.StdEncoding.DecodeString(item.Input.Value)
	if err != nil {
		t.Fatal(err)
	}

	text, err := reminderstext.Decode(data)

	if item.Error != nil {
		checkReminderTextFailure(t, item.Error, err)

		return
	}

	if err != nil {
		t.Fatal(err)
	}

	if item.Result == nil || item.Result.Encoding != testBase64Encoding ||
		base64.StdEncoding.EncodeToString([]byte(text)) != item.Result.Value {
		t.Fatalf("decoded text bytes changed: %q", text)
	}
}

func checkReminderTextFailure(t *testing.T, expected *reminderTextFailure, err error) {
	t.Helper()

	var failure *reminderstext.DecodeError

	if !errors.As(err, &failure) || failure.Unwrap() == nil || failure.ReferenceClass() != expected.Type {
		t.Fatalf("document failure changed: %v", err)
	}

	if expected.Type == "error" {
		var corrupt flate.CorruptInputError

		if !errors.As(failure, &corrupt) {
			t.Fatalf("lost corrupt DEFLATE cause: %v", err)
		}
	} else if failure.Error() != expected.Message {
		t.Fatalf("document failure message changed: %v", err)
	}
}

func checkReminderTextProvenance(t *testing.T, corpus reminderTextCorpus) {
	t.Helper()

	if corpus.Source.URL != "https://github.com/timlaing/pyicloud.git" ||
		corpus.Source.Commit != "e2e44ab875d47dab4475096021da60030f26c35e" ||
		corpus.Evidence != "synthetic; implementation-derived" {
		t.Fatal("Reminders text corpus provenance changed")
	}
}
