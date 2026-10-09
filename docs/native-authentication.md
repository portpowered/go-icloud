# Native authentication parity

The native SDK performs authentication without a Python runtime. Authentication is request scoped: inputs and returned `NativeAuthState` own credentials, cookie scopes, provider options and discovery. Treat that entire state as secret. `ClientError` exposes exact response evidence only through explicit accessors; its normal message omits credentials and response content.

The behavioral reference is pyicloud commit `e2e44ab875d47dab4475096021da60030f26c35e`. Fixtures under `tests/replay/fixtures/synthetic/http/auth-*` are synthetic and implementation derived, rather than private captures. SRP and HID proof transcripts bind cryptographic outputs and protocol frames separately from HTTP scenarios.

| Source authentication surface | Native SDK boundary |
| --- | --- |
| `authenticate` | `Authenticate`, including cached validation, token refresh, one-factor service login, SRP s2k/s2k_fo and paused MFA |
| `get_auth_status` | `GetAuthenticationStatus` |
| `logout` | `Logout`, including remote trust/session scope and local preservation |
| `trusted_devices` | `ListTrustedDevices`, preserving detached device metadata |
| `send_verification_code` | `SendTwoStepCode` |
| `validate_verification_code` | `VerifyTwoStepCode` |
| `use_existing_trusted_device_code` | `UseExistingTrustedDeviceCode`; close an explicit bridge session first |
| `request_2fa_code` | `RequestTwoFactorCode` for SMS/key selection; `OpenNativeBridgeSession` owns bridge prompting |
| `validate_2fa_code` | `VerifyTwoFactorCode` or explicit bridge session verification |
| `trust_session` | `TrustSession` |
| `fido2_devices` | `ListSecurityKeyDevices` |
| `confirm_security_key` | `ConfirmSecurityKey`, with native HID provider or injected `SecurityKeyAuthenticator` |
| `requires_2sa`, `requires_2fa`, `is_trusted_session` | Typed result flags |
| `two_factor_delivery_method`, `two_factor_delivery_notice`, `security_key_names` | Returned state delivery fields and normalized challenge |
| `session` | Caller-owned credentials, scoped cookies and ordered response metadata |
| `webservices`, `get_webservice_url` | Generated account data and typed discovered service origins in `AuthContext` |
| `account_name`, `is_china_mainland` | Request/state account name and caller context region |

`RequestPCSAccess` and explicit term acceptance complete the authentication support needed by Photos. PCS waits, HTTP, entropy and hardware access are injectable. The native SRP implementation uses RFC5054 group 2048 and Source SHA256 proofs; protocol and proof vectors are stored as sanitized offline data.

`VerifySecurityKey` binds WebAuthn client type, origin, challenge, allow-list identity and RP hash before submitting the generated Apple assertion payload. Apple verifies the assertion signature. The positive HTTP fixture uses synthetic assertion bytes and the exact pinned Source submission method plus trust/token login; it does not claim a real device signature or live Apple acceptance. Hardware algorithm/frame proof is maintained separately in `docs/authentication-security-key.md` and dependency tests.

LIB-05 evidence boundaries remain explicit: offline replay proves the pinned implementation behavior for declared responses; no synthetic test proves availability of current Apple services. SDK bridge sessions are explicit lifecycle objects, while Python retains a bridge inside its account object. Failed-session state and ordered HTTP evidence allow callers to resume or choose SMS without losing rotations.
