// Package command implements the CLI using public SDK operations.
package command

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const defaultTimeout = 60 * time.Second

var (
	errArguments = errors.New("provide --session <private AuthContext JSON> and a read command")
	errSession   = errors.New("cannot load authentication context")
	errCommand   = errors.New("unsupported read command")
)

type options struct {
	session   string
	node      string
	family    bool
	timeout   time.Duration
	operation string
}

// Run loads caller-owned authentication and executes one bounded SDK read.
// It neither stores credentials nor prints response headers or opaque session state.
func Run(ctx context.Context, client icloud.Client, args []string, output, diagnostic io.Writer) error {
	config, err := parse(args, diagnostic)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}

	if err != nil {
		return err
	}

	auth, err := loadSession(config.session)
	if err != nil {
		return err
	}

	requestContext, cancel := context.WithTimeout(ctx, config.timeout)
	defer cancel()

	result, err := read(requestContext, client, auth, config)
	if err != nil {
		return fmt.Errorf("read service: %w", err)
	}

	return writeResult(output, result)
}

func parse(args []string, diagnostic io.Writer) (options, error) {
	var config options

	flags := flag.NewFlagSet("go-icloud", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	flags.StringVar(&config.session, "session", "", "Private JSON containing an icloud.AuthContext")
	flags.StringVar(&config.node, "node", "", "Drive node identifier for drive-node")
	flags.BoolVar(&config.family, "family", false, "Include family devices for findmy")
	flags.DurationVar(&config.timeout, "timeout", defaultTimeout, "Request deadline")

	flags.Usage = func() {
		_, _ = fmt.Fprintln(diagnostic, "Usage: go-icloud [flags] <command>\nCommands: account-devices, account-family, "+
			"account-storage, account-plan, drive-libraries, drive-node, findmy")

		flags.PrintDefaults()
	}

	err := flags.Parse(args)
	if err != nil {
		return config, fmt.Errorf("parse command: %w", err)
	}

	if flags.NArg() != 1 || config.session == "" || config.timeout <= 0 {
		return config, errArguments
	}

	config.operation = flags.Arg(0)

	return config, nil
}

func loadSession(path string) (icloud.AuthContext, error) {
	var auth icloud.AuthContext

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return auth, &SessionError{Cause: err}
	}

	err = json.Unmarshal(data, &auth)
	if err != nil {
		return auth, &SessionError{Cause: err}
	}

	return auth, nil
}

func read(ctx context.Context, client icloud.Client, auth icloud.AuthContext, config options) (any, error) {
	switch config.operation {
	case "account-devices":
		return wrap(client.GetAccountDevices(ctx, icloud.GetAccountDevicesRequest{Auth: auth}))
	case "account-family":
		return wrap(client.GetAccountFamily(ctx, icloud.GetAccountFamilyRequest{Auth: auth}))
	case "account-storage":
		return wrap(client.GetAccountStorage(ctx, icloud.GetAccountStorageRequest{Auth: auth}))
	case "account-plan":
		return wrap(client.GetAccountPlanSummary(ctx, icloud.GetAccountPlanSummaryRequest{Auth: auth}))
	case "drive-libraries":
		return wrap(client.ListDriveLibraries(ctx, icloud.ListDriveLibrariesRequest{Auth: auth}))
	case "drive-node":
		return wrap(client.GetDriveNode(ctx, icloud.GetDriveNodeRequest{Auth: auth, NodeID: config.node, ShareID: nil}))
	case "findmy":
		return findMy(ctx, client, auth, config.family)
	default:
		return nil, errCommand
	}
}

func findMy(ctx context.Context, client icloud.Client, auth icloud.AuthContext, family bool) (any, error) {
	session, err := client.OpenFindMySession(ctx, icloud.OpenFindMySessionRequest{Auth: auth, IncludeFamily: family},
		icloud.WithFindMyMonitorInterval(0))
	if err != nil {
		return nil, fmt.Errorf("discover Find My devices: %w", err)
	}

	snapshot, snapshotErr := session.Snapshot()

	closeErr := session.Close()

	err = errors.Join(snapshotErr, closeErr)
	if err != nil {
		return nil, fmt.Errorf("finish Find My session: %w", err)
	}

	return snapshot.Devices, nil
}

func writeResult(output io.Writer, result any) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode service result: %w", err)
	}

	var object map[string]json.RawMessage
	if json.Unmarshal(data, &object) == nil && object != nil {
		delete(object, "metadata")
		result = object
	}

	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")

	err = encoder.Encode(result)
	if err != nil {
		return fmt.Errorf("write service result: %w", err)
	}

	return nil
}

// SessionError identifies unreadable private session data without disclosing it.
type SessionError struct {
	// Cause retains the file or JSON parsing error for explicit inspection.
	Cause error
}

// Error returns a safe display message.
func (failure *SessionError) Error() string { return errSession.Error() }

// Unwrap retains the original failure.
func (failure *SessionError) Unwrap() error { return failure.Cause }

func wrap(result any, err error) (any, error) {
	if err != nil {
		return nil, fmt.Errorf("SDK read: %w", err)
	}

	return result, nil
}
