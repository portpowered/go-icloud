// Command modulecheck verifies every tracked production Go module and formatted source file.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	errReplacement = errors.New("published modules must not contain replace directives")
	errFormatting  = errors.New("tracked Go source differs from gofmt output")
	errNoModules   = errors.New("no tracked Go modules")
)

type moduleDocument struct {
	//nolint:tagliatelle // Go's module-edit JSON owns this fixed capitalized property.
	Replace []json.RawMessage `json:"Replace"`
}

func main() {
	root := flag.String("root", ".", "repository root")

	flag.Parse()

	err := audit(context.Background(), *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func audit(ctx context.Context, root string) error {
	files, err := trackedFiles(ctx, root)
	if err != nil {
		return err
	}

	modules := 0

	for _, path := range files {
		switch {
		case strings.HasSuffix(path, ".go"):
			err = checkFormatting(ctx, root, path)
		case filepath.Base(path) == "go.mod":
			modules++
			err = checkModule(ctx, filepath.Join(root, filepath.Dir(path)))
		}

		if err != nil {
			return err
		}
	}

	if modules == 0 {
		return errNoModules
	}

	return nil
}

func trackedFiles(ctx context.Context, root string) ([]string, error) {
	output, err := run(ctx, root, "git", "ls-files", "-z", "--", "*.go", "go.mod", "**/go.mod")
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00"), nil
}

func checkFormatting(ctx context.Context, root, path string) error {
	output, err := run(ctx, root, "gofmt", "-l", path)
	if err != nil {
		return err
	}

	if len(output) != 0 {
		return fmt.Errorf("%w: %s", errFormatting, path)
	}

	return nil
}

func checkModule(ctx context.Context, directory string) error {
	output, err := run(ctx, directory, "go", "mod", "edit", "-json")
	if err != nil {
		return err
	}

	var module moduleDocument

	err = json.Unmarshal(output, &module)
	if err != nil {
		return fmt.Errorf("decode module %s: %w", directory, err)
	}

	if len(module.Replace) != 0 {
		return fmt.Errorf("%w: %s", errReplacement, directory)
	}

	_, err = run(ctx, directory, "go", "mod", "tidy", "-diff")
	if err != nil {
		return err
	}

	_, err = run(ctx, directory, "go", "mod", "verify")

	return err
}

func run(ctx context.Context, directory, executable string, arguments ...string) ([]byte, error) {
	// #nosec G204 -- fixed verification executables receive separate arguments; no shell is invoked.
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = directory

	command.Env = append(os.Environ(), "GOWORK=off")

	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %s in %s: %w: %s", executable, strings.Join(arguments, " "), directory, err, output)
	}

	return output, nil
}
