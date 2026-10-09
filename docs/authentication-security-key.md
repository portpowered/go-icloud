# Native security key authentication

`ListSecurityKeyDevices` discovers FIDO HID authenticators. `ConfirmSecurityKey`
selects the requested device, or the first discovered device, creates the source
`https://apple.com` WebAuthn assertion, and submits its proof through the existing
authentication flow. Cancel the request context to interrupt a touch or biometric
prompt. The ceremony owns and closes its HID handle, including on cancellation.

The production adapter pins `github.com/telesma-app/hid` v0.12.1. It uses native
Windows and Linux HID interfaces and macOS IOKit through purego, without cgo.
This dependency requires Go 1.25, so the SDK minimum is Go 1.25.0. Raw FIDO HID
access on recent Windows versions can require elevation, as it does in Python's
HID implementation. Device discovery and live ceremonies depend on operating
system permissions and attached hardware.

The adapter implements CTAPHID allocation, fragmented messages, keepalive,
contention retry, cancel, and CTAP2 assertions. It matches Python's credential
probing and selection, chunk reduction, multiple assertion draining, CTAP1/U2F
fallback, and user presence polling. Built-in user verification supports both
PIN/UV protocols: P-256 ECDH, SHA-256 or HKDF, AES-CBC token decryption and HMAC
request authentication. The source's default PIN callback returns no PIN;
the adapter reports `securitykey.ErrPINRequired` for that same condition.

`WithSecurityKeyAuthenticator` replaces hardware behavior at the SDK boundary.
The dependency package also accepts an explicit `Backend`, `Connection`, entropy
reader and cancellable `Waiter`, covering discovery, open, read, write, close and
retry timing. Injected backends and entropy readers must support concurrent
calls. Each assertion opens a separate handle and retains no account credentials
or authorization state (API-13, GO-08, GO-09).

Wire structures, CBOR integer keys, fixed commands and protocol values are
generated from `api/external/securitykey-models.openapi.yaml` (SCHEMA-16). Unknown
authenticator fields remain compatible: canonical validation includes the whole
CBOR message before the known fields are projected. Failures preserve stage,
status and cause without including credential IDs or secret message contents.

The fixtures under `tests/replay/fixtures/securitykey` are explicitly synthetic,
generated with the pinned Python environment's `fido2.cbor`, `CollectedClientData`
and PIN/UV protocol implementations. They are paired HID transcripts, with exact
request frames checked before replies, and separate deterministic cryptographic
vectors. They contain fixed test scalars and invented assertions, never private
captures. They verify source compatibility offline; they do not establish that
an actual authenticator accepted a live Apple challenge. The SDK binds the proof
to the relying party, requested credential and presence flags; Apple verifies
the authenticator signature using the registered credential's public key.
