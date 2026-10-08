// Command go-icloud reads iCloud services through the public Go SDK.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client, err := icloud.New()
	if err == nil {
		err = command.Run(ctx, client, os.Args[1:], os.Stdout, os.Stderr)
	}

	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "go-icloud:", err)

		return 1
	}

	return 0
}
