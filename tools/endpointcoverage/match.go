package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	providerURLTemplate = "{provider-returned-url}"
	albumLocation       = "{album-location}"
)

var placeholderPattern = regexp.MustCompile(`\{[^{}]+\}`)

var (
	errTemplate         = errors.New("invalid endpoint template")
	errProviderTemplate = errors.New("provider URL template must occupy the entire route")
	errAlbumTemplate    = errors.New("album location must prefix a route suffix")
	errParameter        = errors.New("unregistered endpoint template parameter")
)

func validateTemplate(path string) error {
	remaining := placeholderPattern.ReplaceAllString(path, "")
	if strings.ContainsAny(remaining, "{}") {
		return fmt.Errorf("%w: %s", errTemplate, path)
	}

	for _, parameter := range placeholderPattern.FindAllString(path, -1) {
		switch parameter {
		case "{dsid}", "{zone}", "{step}":
		case providerURLTemplate:
			if path != providerURLTemplate {
				return errProviderTemplate
			}
		case albumLocation:
			if !strings.HasPrefix(path, albumLocation+"/") {
				return errAlbumTemplate
			}
		default:
			return fmt.Errorf("%w: %s", errParameter, parameter)
		}
	}

	return nil
}

func matchesRoute(template string, request wireRequest, document scenario, index int) bool {
	if template == providerURLTemplate {
		return referencedURL(request.Origin+request.Path, document, index, false)
	}

	if strings.HasPrefix(template, albumLocation+"/") {
		suffix := strings.TrimPrefix(template, albumLocation)
		if !strings.HasSuffix(request.Path, suffix) {
			return false
		}

		location := request.Origin + strings.TrimSuffix(request.Path, suffix)

		return referencedURL(location, document, index, true)
	}

	pattern := regexp.QuoteMeta(template)
	for _, parameter := range placeholderPattern.FindAllString(template, -1) {
		pattern = strings.ReplaceAll(pattern, regexp.QuoteMeta(parameter), `[^/]+`)
	}

	compiled, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		return false
	}

	return compiled.MatchString(request.Path)
}

func referencedURL(target string, document scenario, index int, prefix bool) bool {
	// Caller-provided asset URLs are supported by the reference download/upload
	// methods. This establishes a URL occurrence, not response-field provenance.
	if containsURL(document.Inputs, target, prefix) {
		return true
	}

	for _, pair := range document.Exchanges[:index] {
		if pair.Response.Body.Encoding != "base64" {
			continue
		}

		payload, err := base64.StdEncoding.DecodeString(pair.Response.Body.Value)
		if err != nil {
			continue
		}

		if containsURL(payload, target, prefix) {
			return true
		}
	}

	return false
}

func containsURL(data []byte, target string, prefix bool) bool {
	var value any

	err := json.Unmarshal(data, &value)
	if err != nil {
		return false
	}

	return valueContainsURL(value, target, prefix)
}

func valueContainsURL(value any, target string, prefix bool) bool {
	switch typed := value.(type) {
	case string:
		return sameURLPath(typed, target, prefix)
	case []any:
		for _, child := range typed {
			if valueContainsURL(child, target, prefix) {
				return true
			}
		}
	case map[string]any:
		for _, child := range typed {
			if valueContainsURL(child, target, prefix) {
				return true
			}
		}
	default:
	}

	return false
}

func sameURLPath(candidate, target string, prefix bool) bool {
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return false
	}

	expected, err := url.Parse(target)
	if err != nil || parsed.Scheme != expected.Scheme || parsed.Host != expected.Host {
		return false
	}

	candidatePath, targetPath := parsed.EscapedPath(), expected.EscapedPath()
	if prefix {
		candidatePath = strings.TrimSuffix(candidatePath, "/")
		targetPath = strings.TrimSuffix(targetPath, "/")
	}

	return candidatePath == targetPath
}
