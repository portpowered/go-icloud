# Native consent, terms, and logout

`RequestPCSAccess` takes the caller's `AuthContext`, `NativeAuthState`, and Apple's
service app name. It first checks web access. When Apple reports ICDRS disabled,
it requests missing consent, polls consent up to ten times, then requests PCS up
to ten times. Only the first PCS request is marked as a direct user action.
Missing consent defaults to granted; missing ICDRS-disabled defaults to false,
matching the pinned reference. Pending cookie responses wait five seconds,
including the final unsuccessful attempt. Consent polling exhaustion proceeds
to the PCS requests, as the reference does.

`WithAuthenticationWait` injects the polling wait for offline verification.
Production waits and injected implementations must honor cancellation. A
cancelled wait returns a typed client error and the preceding response metadata.
Other refusals retain exact final response bytes, status, headers, cookie scope,
and prior metadata. Cookie polling exhaustion has the timeout classification.

`Authenticate` accepts updated terms only when the caller sets `AcceptTerms`.
The flow fetches the terms version, accepts that exact version, then repeats the
original account login. Service one-factor login uses its original credentials
payload for that repetition and validates the resulting web cookie. Missing
versions and unapproved terms return the terms-required error classification.

`Logout` takes caller-owned state and attempts remote logout when both the
account identifier and web authentication cookie exist. `KeepTrusted` and
`AllSessions` select the provider flags. `PreserveLocalSession` retains returned
credentials; otherwise the result clears authentication-derived state.
`RemoteConfirmed` reports the provider's success acknowledgement independently
of `LocalCleared`. Remote refusal or malformed acknowledgement leaves remote
confirmation false, with all received response metadata retained. Context
cancellation propagates before local state is cleared. The library does not own
or remove caller persistence files.

Paired fixtures under `tests/replay/fixtures/synthetic/http` are synthetic,
implementation-derived evidence from the pinned Python reference. They verify
complete outgoing requests and response order for seven PCS cases, seven logout
cases, three terms cases, and service one-factor login. They are not live account
captures.
