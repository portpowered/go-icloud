package command

import (
	"fmt"
	"io"
)

func printUsage(output io.Writer) {
	for _, line := range []string{
		"Usage: go-icloud [flags] <command>",
		"Authentication: login, renew, auth-status, auth-challenge, trust, logout, pcs-access, resume, credentials-export",
		"MFA: mfa-request, mfa-verify, mfa-bridge, mfa-existing-code, mfa-security-keys, mfa-security-key, mfa-devices,",
		"     mfa-send-two-step, mfa-verify-two-step",
		"Account: account-devices, account-family, account-storage, account-plan",
		"Drive: drive-libraries, drive-node",
		"Find My: findmy, findmy-device, findmy-sound, findmy-message, findmy-lost, findmy-erase",
		"Reminders: reminder-legacy-snapshot, reminder-zones, reminder-lists, reminder, reminder-sync, reminder-changes,",
		"           reminders, reminder-snapshot, reminder-tags, reminder-attachments, reminder-recurrence-rules, reminder-alarms",
		"           reminder-create, reminder-update, reminder-delete, reminder-location-add,",
		"           reminder-hashtag-create, reminder-hashtag-update, reminder-hashtag-delete,",
		"           reminder-recurrence-create, reminder-recurrence-update, reminder-recurrence-delete,",
		"           reminder-attachment-create, reminder-attachment-update, reminder-attachment-delete",
		"Photos: photos-status, photo-albums, photo-count, photo-assets, photo, photo-download,",
		"        photo-album-create, photo-album-rename, photo-album-delete, photo-album-add, photo-favorite, photo-delete,",
		"        photo-upload, photo-upload-file, photo-upload-reserve, photo-upload-send, photo-upload-register",
	} {
		_, _ = fmt.Fprintln(output, line)
	}
}
