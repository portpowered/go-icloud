// Command protogen reproduces bridge protobuf models with the pinned compiler and plugin.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	compilerVersion = "libprotoc 31.1"
	pluginPackage   = "google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"
	sourceFile      = "api/external/bridge-proto/bridge.proto"
	outputFile      = "pkg/dependencymodels/bridgepb/bridge.pb.go"
	moduleOption    = "--go_opt=module=github.com/portpowered/go-icloud"
)

var errCompilerVersion = errors.New("bridge generation requires protoc 31.1")

func main() {
	root := flag.String("root", ".", "Repository root")

	flag.Parse()

	err := generate(context.Background(), *root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(ctx context.Context, root string) (resultErr error) {
	version, err := exec.CommandContext(ctx, "protoc", "--version").Output()
	if err != nil {
		return fmt.Errorf("inspect pinned protobuf compiler: %w", err)
	}

	if strings.TrimSpace(string(version)) != compilerVersion {
		return errCompilerVersion
	}
	directory, err := os.MkdirTemp("", "icloud-protogen-")
	if err != nil {
		return fmt.Errorf("create private generator directory: %w", err)
	}

	defer func() {
		cleanupErr := os.RemoveAll(directory)
		if cleanupErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove private generator directory: %w", cleanupErr))
		}
	}()

	install := exec.CommandContext(ctx, "go", "install", pluginPackage)

	install.Env = append(os.Environ(), "GOBIN="+directory, "GOWORK=off")
	output, installErr := install.CombinedOutput()
	if installErr != nil {
		return fmt.Errorf("install pinned protobuf plugin: %w: %s", installErr, output)
	}

	return compile(ctx, root, directory)
}

func compile(ctx context.Context, root, directory string) error {
	plugin := filepath.Join(directory, "protoc-gen-go")
	if runtime.GOOS == "windows" {
		plugin += ".exe"
	}

	_, err := os.Stat(plugin)
	if err != nil {
		return fmt.Errorf("inspect private pinned protobuf plugin: %w", err)
	}

	// Protoc resolves protoc-gen-go from this private directory before the host PATH.
	// The compiler and complete argument vector remain fixed; no shell interprets either path.
	command := exec.CommandContext(ctx, "protoc", "--go_out=.", moduleOption, sourceFile)

	command.Env = append(os.Environ(), "PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	command.Dir = filepath.Clean(root)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("generate schema-owned bridge protobuf: %w: %s", err, output)
	}

	_, err = os.Stat(filepath.Join(root, outputFile))
	if err != nil {
		return fmt.Errorf("inspect generated bridge models: %w", err)
	}

	return nil
}
