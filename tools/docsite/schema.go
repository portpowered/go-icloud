package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

func schemaLinks(root string) ([]string, error) {
	result := []string{}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk schemas: %w", walkErr)
		}

		if entry.IsDir() {
			return nil
		}

		if strings.HasSuffix(file, ".asyncapi.yaml") {
			links, err := asyncAPILinks(file)
			if err != nil {
				return err
			}

			result = append(result, links...)

			return nil
		}

		if !strings.HasSuffix(file, ".openapi.yaml") {
			return nil
		}

		document, err := loader.LoadFromFile(file)
		if err != nil {
			return fmt.Errorf("load schema %s: %w", file, err)
		}

		data, err := json.Marshal(document)
		if err != nil {
			return fmt.Errorf("inspect schema %s: %w", file, err)
		}

		var tree any

		err = json.Unmarshal(data, &tree)
		if err != nil {
			return fmt.Errorf("decode schema links: %w", err)
		}

		result = append(result, collectExternalDocs(tree)...)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory schema links: %w", err)
	}

	return result, nil
}

func asyncAPILinks(file string) ([]string, error) {
	directory, err := os.OpenRoot(filepath.Dir(file))
	if err != nil {
		return nil, fmt.Errorf("open channel schema root: %w", err)
	}

	defer func() { _ = directory.Close() }()

	data, err := readSiteFile(directory, filepath.Base(file))
	if err != nil {
		return nil, fmt.Errorf("read channel schema: %w", err)
	}

	var tree any

	err = yaml.Unmarshal(data, &tree)
	if err != nil {
		return nil, fmt.Errorf("decode channel schema: %w", err)
	}

	return collectExternalDocs(tree), nil
}

func collectExternalDocs(value any) []string {
	result := []string{}

	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if key == "externalDocs" {
				if document, ok := child.(map[string]any); ok {
					if address, valid := document["url"].(string); valid {
						result = append(result, address)
					}
				}
			}

			result = append(result, collectExternalDocs(child)...)
		}
	case []any:
		for _, child := range node {
			result = append(result, collectExternalDocs(child)...)
		}
	}

	return result
}
