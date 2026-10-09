package command

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var errPrivateIdentity = errors.New("cannot identify private session owner")

func protectPrivateFile(ctx context.Context, path string) error {
	output, err := exec.CommandContext(ctx, "whoami", "/user", "/fo", "csv", "/nh").Output()
	if err != nil {
		return fmt.Errorf("identify private file owner: %w", err)
	}

	record, err := csv.NewReader(strings.NewReader(strings.TrimSpace(string(output)))).Read()
	if err != nil {
		return fmt.Errorf("read private file owner: %w", err)
	}

	if len(record) != 2 || !strings.HasPrefix(record[1], "S-1-") {
		return errPrivateIdentity
	}

	//nolint:gosec // Fixed executable/arguments, no shell; path is our empty temporary file and SID is the OS owner.
	err = exec.CommandContext(ctx, "icacls", path, "/inheritance:r", "/grant:r", "*"+record[1]+":F").Run()
	if err != nil {
		return fmt.Errorf("protect private session file: %w", err)
	}

	return nil
}
