package command

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func authenticationFlags(flags *flag.FlagSet, config *options) {
	flags.StringVar(&config.phoneID, "phone-id", "", "Identifier returned by auth-challenge for SMS verification")
	flags.StringVar(&config.trustedDeviceID, "trusted-device", "",
		"Identifier returned by mfa-devices for two-step verification")
	flags.BoolVar(&config.secretStdin, "secret-stdin", false, "Read password or verification code from standard input")
	flags.BoolVar(&config.acceptTerms, "accept-terms", false, "Accept updated provider terms during login")
	flags.BoolVar(&config.china, "china", false, "Use Mainland China authentication on first login")
	flags.BoolVar(&config.keepTrusted, "keep-trusted", false, "Keep trusted browser registration during logout")
	flags.BoolVar(&config.allSessions, "all-sessions", false, "Revoke all sessions during explicit logout")
	flags.StringVar(&config.securityKeyID, "security-key", "",
		"Select a local security-key identifier from mfa-security-keys")
	flags.StringVar(&config.authService, "service", "", "Provider service for explicit one-factor login or pcs-access")
}

func defaultSessionPath() (string, error) {
	if path := os.Getenv("GO_ICLOUD_SESSION"); path != "" {
		return path, nil
	}

	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find private configuration directory: %w", err)
	}

	return filepath.Join(directory, "go-icloud", "session.json"), nil
}

func authenticationCommand(operation string) bool {
	switch operation {
	case authLoginCommand, authRenewCommand, authStatusCommand, authChallengeCommand,
		authMFARequestCommand, authMFAVerifyCommand, authMFADevicesCommand,
		authMFASendTwoStepCommand, authMFAVerifyTwoStepCommand, authSecurityKeysCommand,
		authSecurityKeyCommand, authBridgeCommand,
		authExistingCodeCommand, authSecurityKeyAssertionCommand, authPCSCommand, authTrustCommand, authLogoutCommand:
		return true
	default:
		return false
	}
}
