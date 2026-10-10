package replay_test

const (
	expectedAccountDataField   = `"accountData"`
	expectedSetCookieHeader    = "Set-Cookie"
	expectedAccountEnvironment = "GO_ICLOUD_ACCOUNT"
	// #nosec G101 -- GO-15: CLI environment variable name, not a credential.
	expectedPasswordEnvironment = "GO_ICLOUD_PASSWORD"
	expectedBridgeCodeFixture   = "../../../../tests/replay/fixtures/synthetic/http/" +
		"auth-bridge-modern-code-204.json"
)

// Independent expectations for synthetic authentication replay controls (LIB-05).
const (
	expectedMFARequestCommand    = "mfa-request"
	expectedMFAVerifyCommand     = "mfa-verify"
	expectedCachedAuthFixture    = "auth-authenticate-cached"
	expectedAuthStatusCommand    = "auth-status"
	expectedMFADevicesCommand    = "mfa-devices"
	expectedPCSAccessCommand     = "pcs-access"
	expectedAuthSetupURL         = "https://setup.icloud.com"
	expectedSyntheticAuthCookie  = "synthetic-cookie"
	expectedReplayResponsesField = `"responses"`
	// #nosec G101 -- GO-15: invented synthetic fixture value, not a credential.
	expectedRotatedSessionToken = "synthetic-rotated-token"
	// #nosec G101 -- GO-15: invented synthetic fixture value, not a credential.
	expectedRotatedTrustToken = "synthetic-rotated-trust"
	// #nosec G101 -- GO-15: invented synthetic fixture value, not a credential.
	expectedInventedPassword          = "invented-password"
	expectedSRPCompleteRefusedFixture = "auth-srp-complete-refused"
	expectedSyntheticDSID             = "synthetic-dsid"
	expectedAuthStateKey              = "auth_state"
	expectedTermsRefusedFixture       = "auth-terms-refused"
	expectedSecurityKeyDevice         = "synthetic-device"
	expectedCallerAssertionControl    = "caller assertion"
)

// Independent fixed values in synthetic authentication replay controls (LIB-05).
const (
	expectedSyntheticSessionValue           = "synthetic-token"
	expectedSyntheticAccountName            = "synthetic@example.invalid"
	expectedCancelledCeremonyControl        = "cancelled ceremony"
	expectedSyntheticTrustValue      string = "synthetic-trust"
	expectedMissingDeviceControl     string = "missing device"
)
