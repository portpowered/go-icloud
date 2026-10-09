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
	if err := generate(context.Background(), *root); err != nil {
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
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove private generator directory: %w", cleanupErr))
		}
	}()
	install := exec.CommandContext(ctx, "go", "install", pluginPackage)
	install.Env = append(os.Environ(), "GOBIN="+directory, "GOWORK=off")
	if output, installErr := install.CombinedOutput(); installErr != nil {
		return fmt.Errorf("install pinned protobuf plugin: %w: %s", installErr, output)
	}
	return compile(ctx, root, directory)
}

func compile(ctx context.Context, root, directory string) error {
	plugin := filepath.Join(directory, "protoc-gen-go")
	if runtime.GOOS == "windows" {
		plugin += ".exe"
	}
	command := exec.CommandContext(ctx, "protoc", "--plugin=protoc-gen-go="+plugin,
		"--go_out=.", moduleOption, sourceFile)
	command.Dir = filepath.Clean(root)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("generate schema-owned bridge protobuf: %w: %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(root, outputFile)); err != nil {
		return fmt.Errorf("inspect generated bridge models: %w", err)
	}
	return nil
}
