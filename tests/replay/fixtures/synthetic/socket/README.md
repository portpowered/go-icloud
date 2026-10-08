# Authentication bridge socket transcripts

These 53 invented scenarios execute the pinned reference's actual
`_RawWebSocketClient` and the bridge's push-token/subscription/push helpers.
They are implementation-derived examples, not captures
from Apple's bridge. HTTP service fixtures remain in the sibling `http` folder.

`portos.socket-scenario.v1` records connection inputs, ordered send/receive/close
events, caller actions, base64 results, typed errors, and observed socket closure.
The runner injects the TCP/TLS factories at the plaintext socket seam. It checks
the destination, server name, timeout, ownership, upgrade bytes, client framing,
decoded payload, event order, complete consumption, and semantic outcome.
Default network access remains forbidden (LIB-05, LIB-18).

The upgrade key must be canonical base64 for 16 bytes. Its accept header is
derived explicitly using RFC 6455's GUID. Client masks may vary over four bytes;
frame opcode, finality, canonical length, exact extent, and decoded payload must
match. Server frame bytes are fixed. Receive chunking and upgrade/frame
coalescing exercise the reference's buffering rather than supplying decoded
messages directly.

Cases cover empty through 64-bit length framing, masked server text, fragmented
binary messages, interleaved ping/pong, split and coalesced receives, peer close,
unsupported opcode, EOF, invalid accept, denied upgrade, and cleanup after a
close-frame send failure.

Bridge-message cases execute the pinned manual protobuf decoder and encoders
through the framed socket seam. They cover token success/missing/rejection,
nonce retry metadata, malformed varints/fields/wire types/base64, subscriptions
with zero/one/three topics, subscription responses, acknowledgments, unknown
messages, foreign-topic acknowledgment and filtering, named/hashed topics,
JSON framing, optional fields, `flowid`, and strict payload-validation failures.
Wait actions bind every monotonic read to nonnegative integer samples in
`monotonic_ticks`, reject decreasing traces and unexpected reads, and require
complete clock consumption even when the reference raises. Timeout cases cover
both immediate expiration and reaching the deadline after an ignored token
message or acknowledged foreign-topic push. These deterministic traces do not
prove long-running wall-clock scheduling or cancellation. Nonce errors include
the server timestamp as a portable semantic field. Acknowledgments must match the
received message ID and topic bytes before the next frame is available.

The three failed-upgrade cases confirm that construction failure does not
explicitly close the created socket. `reference_socket_closed: false` records
the absence of a close call. This seam does not model object destruction or
garbage collection and cannot prove a persistent live socket leak.
The Go implementation must close a connection on upgrade failure and document
that deliberate correction when porting these cases.

This seam does not verify TLS encryption/certificates, complete protobuf
variants, cryptographic bridge steps, the full login lifecycle, or live Apple
behavior.
Those obligations remain open. Private historical login captures do not contain
socket frames and cannot be treated as full authentication replay.
