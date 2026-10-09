//go:build !windows

package command

import "context"

func protectPrivateFile(context.Context, string) error { return nil }
