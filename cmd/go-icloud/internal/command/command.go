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

const reminderCommand = "reminder"

var (
	errArguments = errors.New("provide a private session and command, or --reference-state <directory> resume")
	errSession   = errors.New("cannot access private session data")
	errCommand   = errors.New("unsupported read command")
	errReminder  = errors.New("provide --reminder <identifier> for reminder")
)

type options struct {
	session          string
	node             string
	reminderID       string
	relatedIDs       []string
	listID           string
	includeCompleted bool
	resultsLimit     *int64
	since            *string
	family           bool
	timeout          time.Duration
	operation        string
	referenceState   string
	saveSession      string
	forceRefresh     bool
	allowUntrusted   bool
}

// Run executes one bounded public SDK operation.
// Resume saves copied credentials privately; console output omits secret session state.
func Run(ctx context.Context, client icloud.Client, args []string, output, diagnostic io.Writer) error {
	config, err := parse(args, diagnostic)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}

	if err != nil {
		return err
	}

	requestContext, cancel := context.WithTimeout(ctx, config.timeout)
	defer cancel()

	if config.operation == "resume" {
		return resumeSession(requestContext, client, config, output)
	}

	auth, err := loadSession(config.session)
	if err != nil {
		return err
	}

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
	flags.StringVar(&config.referenceState, "reference-state", "", "Existing reference login directory for resume")
	flags.StringVar(&config.saveSession, "save-session", "", "Private native-session destination for resume")
	flags.BoolVar(&config.forceRefresh, "force-refresh", false, "Skip cookie validation during resume")
	flags.BoolVar(&config.allowUntrusted, "allow-untrusted", false, "Save paused MFA discovery during resume")
	flags.StringVar(&config.node, "node", "", "Drive node identifier for drive-node")
	flags.StringVar(&config.reminderID, "reminder", "", "Raw or complete reminder identifier for reminder")
	flags.Func("since", "Optional cursor for reminder-changes; omission starts an initial read", func(value string) error {
		config.since = &value

		return nil
	})
	flags.BoolVar(&config.family, "family", false, "Include family devices for findmy")
	flags.Func("related-id", "Repeatable identifier for reminder related-record reads", func(value string) error {
		config.relatedIDs = append(config.relatedIDs, value)

		return nil
	})
	reminderQueryFlags(flags, &config)
	flags.DurationVar(&config.timeout, "timeout", defaultTimeout, "Request deadline")

	flags.Usage = func() {
		_, _ = fmt.Fprintln(diagnostic, "Usage: go-icloud [flags] <command>\nCommands: account-devices, account-family, "+
			"account-storage, account-plan, drive-libraries, drive-node, findmy, "+
			"reminder-zones, reminder-lists, reminder, reminder-sync, reminder-changes, reminders, reminder-snapshot, "+
			"reminder-tags, reminder-attachments, reminder-recurrence-rules, reminder-alarms, "+
			"photos-status, photo-albums, resume")

		flags.PrintDefaults()
	}

	err := flags.Parse(args)
	if err != nil {
		return config, fmt.Errorf("parse command: %w", err)
	}

	if flags.NArg() != 1 || config.timeout <= 0 {
		return config, errArguments
	}

	config.operation = flags.Arg(0)
	if config.operation == reminderCommand && config.reminderID == "" {
		return config, errReminder
	}

	if config.session == "" && (config.operation != "resume" || config.referenceState == "") {
		return config, errArguments
	}

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

	if auth.ClientID == "" {
		var saved icloud.ResumeSessionResult

		err = json.Unmarshal(data, &saved)
		if err != nil {
			return auth, &SessionError{Cause: err}
		}

		auth = saved.Auth
	}

	return auth, nil
}

func read(ctx context.Context, client icloud.Client, auth icloud.AuthContext, config options) (any, error) {
	switch config.operation {
	case "reminder-zones", "reminder-lists", reminderCommand, "reminder-sync", "reminder-changes",
		"reminders", "reminder-snapshot",
		"reminder-tags", "reminder-attachments", "reminder-recurrence-rules", "reminder-alarms":
		return readReminders(ctx, client, auth, config)
	case "photos-status", "photo-albums":
		return readPhotos(ctx, client, auth, config.operation)
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

func readReminders(ctx context.Context, client icloud.Client, auth icloud.AuthContext, config options) (any, error) {
	switch config.operation {
	case "reminder-zones":
		return wrap(client.ListReminderZones(ctx, icloud.ListReminderZonesRequest{Auth: auth}))
	case reminderCommand:
		return wrap(client.GetReminder(ctx, icloud.GetReminderRequest{Auth: auth, ReminderID: config.reminderID}))
	case "reminder-sync":
		return wrap(client.GetReminderSyncCursor(ctx, icloud.GetReminderSyncCursorRequest{Auth: auth}))
	case "reminder-changes":
		return wrap(client.ListReminderChanges(ctx, icloud.ListReminderChangesRequest{Auth: auth, Since: config.since}))
	case "reminders":
		return wrap(client.ListReminders(ctx, icloud.ListRemindersRequest{Auth: auth, ListID: config.listID,
			IncludeCompleted: &config.includeCompleted, ResultsLimit: config.resultsLimit}))
	case "reminder-snapshot":
		return wrap(client.ListReminderSnapshot(ctx,
			icloud.ListReminderSnapshotRequest{Auth: auth, ListID: &config.listID}))
	case "reminder-lists":
		return wrap(client.ListReminderLists(ctx, icloud.ListReminderListsRequest{Auth: auth}))
	default:
		return readReminderRelated(ctx, client, auth, config)
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
		delete(object, "responses")
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

func readReminderRelated(ctx context.Context, client icloud.Client,
	auth icloud.AuthContext, config options,
) (any, error) {
	switch config.operation {
	case "reminder-tags":
		return wrap(client.ListReminderTags(ctx, icloud.ListReminderTagsRequest{Auth: auth, IDs: config.relatedIDs}))
	case "reminder-attachments":
		return wrap(client.ListReminderAttachments(ctx,
			icloud.ListReminderAttachmentsRequest{Auth: auth, IDs: config.relatedIDs}))
	case "reminder-recurrence-rules":
		return wrap(client.ListReminderRecurrenceRules(ctx,
			icloud.ListReminderRecurrenceRulesRequest{Auth: auth, IDs: config.relatedIDs}))
	case "reminder-alarms":
		return wrap(client.ListReminderAlarms(ctx, icloud.ListReminderAlarmsRequest{Auth: auth, IDs: config.relatedIDs}))
	default:
		return nil, errCommand
	}
}

func readPhotos(ctx context.Context, client icloud.Client, auth icloud.AuthContext, operation string) (any, error) {
	if operation == "photos-status" {
		return wrap(client.GetPhotosStatus(ctx, icloud.GetPhotosStatusRequest{Auth: auth}))
	}
	return wrap(client.ListPhotoAlbums(ctx, icloud.ListPhotoAlbumsRequest{Auth: auth}))
}
