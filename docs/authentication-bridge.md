# Trusted-device authentication bridge

The native trusted-device bridge follows the pinned Python implementation at
`e2e44ab875d47dab4475096021da60030f26c35e`. Its public account state is generated
from `api/client-models.openapi.yaml`; bootstrap and push payloads are generated
from `api/external/bridge-models.openapi.yaml`. The protobuf exchange inventory
is reconstructed from that implementation in
`api/external/bridge-proto/bridge.proto`, with its socket channels described in
`api/bridge.asyncapi.yaml`. This is reference-derived evidence, not a
provider-published protocol or a private account capture (SCHEMA-08).

Call `GetAuthenticationChallenge`, then `RequestTwoFactorCode`. A bridge route
leaves the prompt pending until `OpenNativeBridgeSession` opens its socket.
Pass the returned account state and its `AuthContext` to the opening request.
The returned session owns this account's credential updates and connection
until verification or `Close` (API-04, API-13). `State` returns copies of the
current account credentials, HTTP response metadata, and challenge progress.
`VerifyCode` returns the final `NativeAuthResult`, including updated credentials.
Verification and cancellation close the socket; explicit `Close` is idempotent.

The modern exchange signs a timestamped nonce with a fresh P-256 key, subscribes
to the selected APNS topic, acknowledges every received push, and follows HTTP
steps 0, 2, 4, and 6. An invalid nonce permits one retry with the server timestamp.
The native prover implements the pinned SPAKE2 transcript, scrypt derivation,
confirmation proofs, and version-0 AES-GCM encrypted verification code. Legacy
transactions use the trusted-device HTTP verifier and close their socket before
trusting the browser. No Python process is required at runtime.

The SDK's existing random reader and clock control entropy and time. A per-session
`WithNativeBridgeDial` option supplies the secured connection at the network
boundary. Its contract requires the caller to establish TLS when connecting to
a real server; offline tests supply a scripted connection. Without this option,
the transport establishes and verifies TLS itself. The socket implementation
handles raw RFC 6455 upgrade, masking, fragmentation, ping/pong, and close frames.

The socket fixture inventory contains 53 synthetic paired scenarios derived
from the pinned source: 18 raw WebSocket transcripts and 35 protobuf/push
transcripts. Replay consumes both outgoing and incoming bytes, including every
ACK and close, and verifies the complete decoded payload. Additional native
tests verify the signed bootstrap, deterministic Python cryptographic vectors,
nonce retry, modern proof exchange, cancellation, overlapping verification,
state copies, and failed confirmations. These fixtures are synthetic evidence;
they do not establish live-account compatibility.

Response headers can contain private credential values. Display client errors
using their safe `Error` method. Inspect response metadata deliberately when
applying rotated credentials after a failed exchange; the session retains those
updates even when a later trust request fails.
