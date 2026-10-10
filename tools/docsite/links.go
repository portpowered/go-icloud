package main

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const siteOrigin = "https://portpowered.github.io"

func checkLinks(site, base string, pages map[string]page, schemaURLs []string) error {
	for address, rendered := range pages {
		for _, target := range rendered.links {
			err := checkLink(site, base, pages, address+"/", target)
			if err != nil {
				return err
			}
		}
	}

	for _, target := range schemaURLs {
		err := checkLink(site, base, pages, base+"/docs/", target)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkLink(site, base string, pages map[string]page, address, target string) error {
	current, err := url.Parse(siteOrigin + address)
	if err != nil {
		return fmt.Errorf("parse current page: %w", err)
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid link on %s: %w", address, err)
	}

	resolved := current.ResolveReference(parsed)
	if resolved.Scheme != "https" && resolved.Scheme != "http" {
		return nil
	}

	if resolved.Host != current.Host {
		return nil
	}

	destination := strings.TrimSuffix(resolved.Path, "/")
	if rendered, exists := pages[destination]; exists {
		if resolved.Fragment != "" && !rendered.ids[resolved.Fragment] {
			return fmt.Errorf("%w on %s: %s", errMissingFragment, address, target)
		}

		return nil
	}

	return checkAsset(site, base, address, destination, target)
}

func checkAsset(site, base, address, destination, target string) error {
	if !strings.HasPrefix(destination, base+"/") {
		return fmt.Errorf("%w on %s: %s", errProjectLink, address, target)
	}

	relative := strings.TrimPrefix(destination, base+"/")
	candidate := filepath.Join(site, filepath.FromSlash(path.Clean(relative)))

	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return fmt.Errorf("%w on %s: %s", errMissingDestination, address, target)
	}

	return nil
}
