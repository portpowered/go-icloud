// Command apiconstants generates protocol constants from canonical service schemas.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/format"
	"go/token"
	"os"
	"regexp"
	"regexp/syntax"
	"sort"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	protocolDirectoryMode     = 0o700
	protocolFileMode          = 0o600
	prefixProbeUUID           = "00000000-0000-4000-8000-000000000000"
	minimumPrefixPatternParts = 4
)

var errConstant = errors.New("invalid or ambiguous protocol constant")

func main() {
	err := generateAccountConstants()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = generateExternalConstants("Auth", "auth")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = generateExternalConstants("Drive", "drive")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = generateExternalConstants("FindMy", "findmy")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = generateExternalConstants("Reminders", "cloudkit")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generateExternalConstants(prefix, modelsName string) error {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	name := strings.ToLower(prefix)

	document, err := loader.LoadFromFile("api/external/" + name + ".openapi.yaml")
	if err != nil {
		return fmt.Errorf("load %s schema: %w", prefix, err)
	}

	err = document.Validate(context.Background())
	if err != nil {
		return fmt.Errorf("validate %s schema: %w", prefix, err)
	}

	models, err := loader.LoadFromFile("api/external/" + modelsName + "-models.openapi.yaml")
	if err != nil {
		return fmt.Errorf("load %s models: %w", prefix, err)
	}

	values, err := externalConstants(document, models)
	if err != nil {
		return err
	}

	formatted, err := renderConstants(values, prefix)
	if err != nil {
		return err
	}

	err = os.WriteFile("internal/protocol/"+name+".gen.go", formatted, protocolFileMode)
	if err != nil {
		return fmt.Errorf("write %s protocol constants: %w", prefix, err)
	}

	return nil
}

func generateAccountConstants() error {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	document, err := loader.LoadFromFile("api/external/account.openapi.yaml")
	if err != nil {
		return fmt.Errorf("load account schema: %w", err)
	}

	err = document.Validate(context.Background())
	if err != nil {
		return fmt.Errorf("validate account schema: %w", err)
	}

	formatted, err := constantsSource(document)
	if err != nil {
		return err
	}

	err = os.MkdirAll("internal/protocol", protocolDirectoryMode)
	if err != nil {
		return fmt.Errorf("create protocol directory: %w", err)
	}

	err = os.WriteFile("internal/protocol/account.gen.go", formatted, protocolFileMode)
	if err != nil {
		return fmt.Errorf("write protocol constants: %w", err)
	}

	return nil
}

func accountConstants(document *openapi3.T) (map[string]string, error) {
	values := make(map[string]string)

	var conflict error

	register := func(name, value string) {
		if _, exists := values[name]; exists || !token.IsIdentifier(name) {
			conflict = fmt.Errorf("%w: %s", errConstant, name)

			return
		}

		values[name] = value
	}

	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			register(operation.OperationID+"Path", path)
			register(operation.OperationID+"Method", strings.ToUpper(method))

			registerOperationMedia(operation, values, register)
			registerInlineParameters(operation, register)
		}
	}

	for name := range document.Components.Headers {
		register("HTTP"+identifier(name)+"Name", name)
	}

	for name, parameter := range document.Components.Parameters {
		register(name+"Name", parameter.Value.Name)
	}

	for name, schema := range document.Components.Schemas {
		err := registerSchemaConstants(name, schema.Value, register)
		if err != nil {
			return nil, err
		}
	}

	for index, server := range document.Servers {
		register(fmt.Sprintf("AccountServer%d", index), server.URL)
	}

	return values, conflict
}

func registerProperties(name string, schema *openapi3.Schema, register func(name, value string)) error {
	for property, field := range schema.Properties {
		register(name+identifier(property), property)

		value, exists := field.Value.Extensions["x-protocol-prefix"]
		if !exists {
			continue
		}

		prefix, valid := value.(string)
		if !valid || prefix == "" || !prefixMatchesPattern(prefix, field.Value.Pattern) {
			return fmt.Errorf("%w: %s prefix must match its anchored pattern", errConstant, name)
		}

		register(name+identifier(property)+"Prefix", prefix)
	}

	return nil
}

func prefixMatchesPattern(prefix, pattern string) bool {
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil || parsed.Op != syntax.OpConcat || len(parsed.Sub) < minimumPrefixPatternParts {
		return false
	}

	literal := parsed.Sub[1]
	if parsed.Sub[0].Op != syntax.OpBeginText || parsed.Sub[len(parsed.Sub)-1].Op != syntax.OpEndText ||
		literal.Op != syntax.OpLiteral || literal.Flags&syntax.FoldCase != 0 || string(literal.Rune) != prefix {
		return false
	}

	compiled, err := regexp.Compile(pattern)

	return err == nil && compiled.MatchString(prefix+prefixProbeUUID)
}

func externalConstants(routes, models *openapi3.T) (map[string]string, error) {
	// The models document owns property names; the route document owns operations and media.
	routeCopy := *routes
	components := *routes.Components
	components.Schemas = nil
	routeCopy.Components = &components

	values, err := accountConstants(&routeCopy)
	if err != nil {
		return nil, err
	}

	properties, err := accountConstants(models)
	if err != nil {
		return nil, err
	}

	for name, value := range properties {
		if _, exists := values[name]; exists {
			return nil, fmt.Errorf("%w: %s", errConstant, name)
		}

		values[name] = value
	}

	return values, nil
}

func registerInlineParameters(operation *openapi3.Operation, register func(name, value string)) {
	for _, parameter := range operation.Parameters {
		if parameter.Ref == "" {
			register(operation.OperationID+identifier(parameter.Value.Name)+"Name", parameter.Value.Name)
		}
	}
}

func registerOperationMedia(operation *openapi3.Operation, values map[string]string,
	register func(name, value string),
) {
	if operation.RequestBody != nil {
		for mediaType := range operation.RequestBody.Value.Content {
			name := "Media" + identifier(mediaType)
			if value, exists := values[name]; !exists || value != mediaType {
				register(name, mediaType)
			}
		}
	}

	for _, response := range operation.Responses.Map() {
		for mediaType := range response.Value.Content {
			name := "Media" + identifier(mediaType)
			if value, exists := values[name]; !exists || value != mediaType {
				register(name, mediaType)
			}
		}
	}
}

func identifier(value string) string {
	var output strings.Builder

	capitalize := true

	for _, character := range value {
		if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			capitalize = true

			continue
		}

		if capitalize {
			character = unicode.ToUpper(character)
			capitalize = false
		}

		output.WriteRune(character)
	}

	return output.String()
}

func constantsSource(document *openapi3.T) ([]byte, error) {
	values, err := accountConstants(document)
	if err != nil {
		return nil, err
	}

	return renderConstants(values, "")
}

func renderConstants(values map[string]string, prefix string) ([]byte, error) {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}

	sort.Strings(names)

	var output bytes.Buffer

	output.WriteString("// Code generated by tools/apiconstants. DO NOT EDIT.\npackage protocol\n\nconst (\n")

	for _, name := range names {
		identifierPrefix := prefix
		if strings.HasPrefix(name, prefix) {
			identifierPrefix = ""
		}

		fmt.Fprintf(&output, "\t%s%s = %q\n", identifierPrefix, name, values[name])
	}

	output.WriteString(")\n")

	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format protocol constants: %w", err)
	}

	return formatted, nil
}

func registerSchemaConstants(name string, schema *openapi3.Schema, register func(name, value string)) error {
	err := registerScalarConstants(name, schema, register)
	if err != nil {
		return err
	}

	return registerProperties(name, schema, register)
}

func registerScalarConstants(name string, schema *openapi3.Schema, register func(name, value string)) error {
	if len(schema.Enum) == 1 {
		if value, valid := schema.Enum[0].(string); valid {
			register(name+"Value", value)
		}
	}

	if schema.Pattern != "" {
		_, err := regexp.Compile(schema.Pattern)
		if err != nil {
			return fmt.Errorf("%w: %s pattern is invalid", errConstant, name)
		}

		register(name+"Pattern", schema.Pattern)
	}

	if value, exists := schema.Extensions["x-protocol-template"]; exists {
		template, valid := value.(string)
		if !valid || !validProtocolTemplate(template, schema.Pattern) {
			return fmt.Errorf("%w: %s template must match its pattern", errConstant, name)
		}

		register(name+"Template", template)
	}

	return nil
}

func validProtocolTemplate(template, pattern string) bool {
	if pattern == "" || !strings.Contains(template, "%s") {
		return false
	}

	probe := strings.ReplaceAll(template, "%s", "synthetic")
	if strings.Contains(probe, "%") {
		return false
	}

	compiled, err := regexp.Compile(pattern)

	return err == nil && strings.HasPrefix(pattern, "^") && strings.HasSuffix(pattern, "$") && compiled.MatchString(probe)
}
