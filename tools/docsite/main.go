// Command docsite verifies links and expected content in the complete static documentation export.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	site := flag.String("site", "site", "static site directory")
	base := flag.String("base", "/go-icloud", "GitHub Pages base path")
	schemas := flag.String("schemas", "api", "canonical schema directory")
	expected := flag.String("expect", "docs/site-expectations.json", "rendered content expectations")

	flag.Parse()

	err := verify(*site, *base, *schemas, *expected)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func verify(site, base, schemas, expected string) error {
	pages, err := readPages(site, base)
	if err != nil {
		return err
	}

	links, err := schemaLinks(schemas)
	if err != nil {
		return err
	}

	err = checkLinks(site, base, pages, links)
	if err != nil {
		return err
	}

	err = checkContent(pages, expected)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(os.Stdout, "Verified %d HTML pages, local links, schema links, and expected content\n",
		len(pages))
	if err != nil {
		return fmt.Errorf("write verification result: %w", err)
	}

	return nil
}
