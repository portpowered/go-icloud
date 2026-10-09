package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
	"golang.org/x/net/html"
)

var errBootstrap = errors.New("missing or malformed HSA2 bootstrap")

// ParseBootstrap parses the first boot_args script from the authentication page.
func ParseBootstrap(body []byte) (*models.BridgeBootstrapDirect, error) {
	payload, err := bootScript(body)
	if err != nil {
		return nil, &Failure{Stage: "bootstrap HTML", Cause: err}
	}
	var envelope models.BridgeBootstrapEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, &Failure{Stage: "bootstrap JSON", Cause: err}
	}
	if envelope.Direct == nil {
		return nil, &Failure{Stage: "bootstrap JSON", Cause: errBootstrap}
	}
	return envelope.Direct, nil
}

func bootScript(body []byte) ([]byte, error) {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	collecting := false
	var payload []byte
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			if !errors.Is(tokenizer.Err(), io.EOF) {
				return nil, fmt.Errorf("tokenize bootstrap: %w", tokenizer.Err())
			}
			if collecting && len(bytes.TrimSpace(payload)) != 0 {
				return payload, nil
			}
			return nil, errBootstrap
		case html.StartTagToken:
			if !collecting {
				collecting = isBootScript(tokenizer.Token())
			}
		case html.TextToken:
			if collecting {
				payload = append(payload, tokenizer.Text()...)
			}
		case html.EndTagToken:
			if collecting && tokenizer.Token().Data == string(models.BootScriptTag) {
				if len(bytes.TrimSpace(payload)) == 0 {
					return nil, errBootstrap
				}
				return payload, nil
			}
		}
	}
}

func isBootScript(token html.Token) bool {
	if token.Data != string(models.BootScriptTag) {
		return false
	}
	for _, attribute := range token.Attr {
		if attribute.Key != string(models.BootClassAttribute) {
			continue
		}
		for _, class := range strings.Fields(attribute.Val) {
			if class == string(models.BootScriptClass) {
				return true
			}
		}
	}
	return false
}

// SocketHost resolves the source-supplied host or its named APNS environment.
func SocketHost(data models.BridgeInitiateData) (string, error) {
	if candidate := valueOrEmpty(data.WebSocketUrl); candidate != "" {
		if strings.Contains(candidate, "://") {
			parsed, err := url.Parse(candidate)
			if err != nil {
				return "", fmt.Errorf("parse bridge host: %w", err)
			}
			if parsed.Hostname() != "" {
				return parsed.Hostname(), nil
			}
		}
		return strings.SplitN(candidate, "/", 2)[0], nil
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
