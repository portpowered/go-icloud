package main

import (
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	linkPattern   = regexp.MustCompile(`(?i)<(?:a|link|script|img|source)\b[^>]*?\b(?:href|src)\s*=\s*["']([^"']*)["']`)
	idPattern     = regexp.MustCompile(`(?i)\bid\s*=\s*["']([^"']*)["']`)
	tagPattern    = regexp.MustCompile(`<[^>]+>`)
	hiddenPattern = regexp.MustCompile(`(?is)<(?:script|style)\b[^>]*>.*?</(?:script|style)>`)
)

type page struct {
	links []string
	ids   map[string]bool
	text  string
}

func readPages(site, base string) (map[string]page, error) {
	pages := make(map[string]page)

	directory, err := os.OpenRoot(site)
	if err != nil {
		return nil, fmt.Errorf("open site root: %w", err)
	}

	defer func() { _ = directory.Close() }()

	err = filepath.WalkDir(site, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk site: %w", walkErr)
		}

		if entry.IsDir() || filepath.Ext(file) != ".html" {
			return nil
		}

		relative, err := filepath.Rel(site, file)
		if err != nil {
			return fmt.Errorf("resolve rendered page: %w", err)
		}

		data, err := readSiteFile(directory, relative)
		if err != nil {
			return err
		}

		address := path.Join(base, filepath.ToSlash(relative))
		address = strings.TrimSuffix(address, "/index.html")
		pages[address] = parsePage(string(data))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inventory rendered pages: %w", err)
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("%w: %s", errEmptySite, site)
	}

	return pages, nil
}

func readSiteFile(directory *os.Root, name string) ([]byte, error) {
	file, err := directory.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open rendered page: %w", err)
	}

	data, readErr := io.ReadAll(file)
	closeErr := file.Close()

	if readErr != nil {
		return nil, fmt.Errorf("read rendered page: %w", readErr)
	}

	if closeErr != nil {
		return nil, fmt.Errorf("close rendered page: %w", closeErr)
	}

	return data, nil
}

func parsePage(data string) page {
	visible := html.UnescapeString(tagPattern.ReplaceAllString(hiddenPattern.ReplaceAllString(data, ""), " "))

	result := page{
		links: []string{}, ids: make(map[string]bool),
		text: strings.Join(strings.Fields(visible), " "),
	}
	for _, match := range linkPattern.FindAllStringSubmatch(data, -1) {
		result.links = append(result.links, html.UnescapeString(match[1]))
	}

	for _, match := range idPattern.FindAllStringSubmatch(data, -1) {
		result.ids[html.UnescapeString(match[1])] = true
	}

	return result
}
