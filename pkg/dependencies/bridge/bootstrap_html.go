package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
	"golang.org/x/net/html"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var errBootstrap = errors.New("missing or malformed HSA2 bootstrap")

// ParseBootstrap parses the first boot_args script from the authentication page.
func ParseBootstrap(body []byte) (*models.BridgeBootstrapDirect, error) {
	payload, err := bootScript(body)
	if err != nil {
		return nil, &ProtocolError{Stage: "bootstrap HTML", Cause: err}
	}

	var envelope models.BridgeBootstrapEnvelope

	err = json.Unmarshal(payload, &envelope)
	if err != nil {
		return nil, &ProtocolError{Stage: stageBootstrapJSON, Cause: err}
	}

	if envelope.Direct == nil {
		return nil, &ProtocolError{Stage: stageBootstrapJSON, Cause: errBootstrap}
	}

	return envelope.Direct, nil
}

func bootScript(body []byte) ([]byte, error) {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	for tokenizer.Next() != html.ErrorToken {
		if isBootScript(tokenizer.Token()) {
			return collectScript(tokenizer)
		}
	}

	return nil, errBootstrap
}

func collectScript(tokenizer *html.Tokenizer) ([]byte, error) {
	var payload []byte

	for {
		tokenType := tokenizer.Next()
		if tokenType == html.TextToken {
			payload = append(payload, tokenizer.Text()...)
		}

		if tokenType == html.ErrorToken || tokenType == html.EndTagToken {
			if !errors.Is(tokenizer.Err(), io.EOF) && tokenizer.Err() != nil {
				return nil, fmt.Errorf("tokenize bootstrap: %w", tokenizer.Err())
			}

			if len(bytes.TrimSpace(payload)) == 0 {
				return nil, errBootstrap
			}

			return payload, nil
		}
	}
}

func isBootScript(token html.Token) bool {
	if token.Data != string(models.BootScriptTag) {
		return false
	}

	for _, attribute := range token.Attr {
		if attribute.Key == string(models.BootClassAttribute) {
			return slices.Contains(strings.Fields(attribute.Val), string(models.BootScriptClass))
		}
	}

	return false
}

// SocketHost resolves the source-supplied host or its named APNS environment.
func SocketHost(data models.BridgeInitiateData) (string, error) {
	if candidate := valueOrEmpty(data.WebSocketUrl); candidate != "" {
		return explicitSocketHost(candidate)
	}

	switch valueOrEmpty(data.ApnsEnvironment) {
	case string(models.ProductionEnvironment):
		return string(models.ProductionHost), nil
	case string(models.SandboxEnvironment):
		return string(models.SandboxHost), nil
	default:
		return "", errBootstrap
	}
}

func explicitSocketHost(candidate string) (string, error) {
	if strings.Contains(candidate, "://") {
		host, err := sourceURLHostname(candidate)
		if err != nil {
			return "", fmt.Errorf("parse bridge host: %w", err)
		}

		if host != "" {
			return lowercaseSocketHostname(host), nil
		}
	}

	first, _, _ := strings.Cut(candidate, "/")

	return first, nil
}

func lowercaseSocketHostname(host string) string {
	name, zone, zoned := strings.Cut(host, "%")
	name = cases.Lower(language.Und).String(name)
	if zoned {
		return name + "%" + zone
	}

	return name
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
