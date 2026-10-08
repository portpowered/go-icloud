# Client design guidance

## Provider client conventions

The extracted library's `pkg/<provider>` package is the public API for one provider.
The synthetic scaffold starts at the module root and must move before release.
Keep its request and result types independent of wire-format structs. Group the supported
operations in the Client interface so callers can see the library surface in
one place.

Use context.Context on every operation that can wait on I/O. Keep
account-specific credentials on each account request so one client can serve
multiple accounts without changing shared authorization state. Return typed
errors that preserve causes and expose a stable failure kind.

Construct the reusable client from named functional options.
Validate endpoints and conflicting options at construction time. Configuration
may hold immutable service settings and injected transports; account tokens
belong in requests or explicit sessions. Do not update shared client fields
when a login or refresh call returns new credentials. Return those credentials
to the caller for storage and later requests.

Audit mutable state inside injected clients as well as SDK fields. Shallow-copying
an `http.Client` preserves its CookieJar pointer. Reject a shared account cookie jar
with an inspectable configuration error or put cookie storage in an explicit
session. Prove isolation with two accounts and complete outbound request assertions;
keep stateless custom transports injectable.

The example Client interface is a provider-specific contract. These design
conventions can be reused by separate provider libraries, but this template has
one implementation and does not validate a shared interface across providers.

## Optional cross-vendor capabilities

Add a cross-vendor capability interface only after at least two independent
provider implementations demonstrate the same operation semantics and failure
behavior. Keep each interface narrow and based on what a client can do, such as
recording retrieval or device control. Do not make every provider satisfy an
umbrella interface when support varies.

Before adding a shared interface, compare the request inputs, result meaning,
authentication needs, cancellation behavior, and error classes across the
implementations. If those differ, keep the APIs provider-specific or define a
smaller capability with behavior both implementations can honor. This template
contains no cross-vendor capability interface.

## Optional long-running sessions

Skip session types when operations are short request/response calls. Use an
explicit session for multi-step login, long-running connections, streams, and
other stateful lifecycles. Pass a context when opening it, document who owns
Close, make repeated close calls safe, and surface terminal errors and
backpressure to the caller. Keep account and connection state in the session
rather than the reusable client. Provide a way for callers to read tokens
produced by a session without reading internal client fields.

## Transport ownership

For interactive authorization, expose a consent URL and typed callback completion
on caller-owned authorization state or an explicit session. Keep the reusable
client stateless. Bind the callback to the original state and exact redirect,
use PKCE when the provider supports it, and complete any required activation
call before reporting success. Callers own callback endpoints; an optional CLI
loopback listener must have an injectable, cancellable lifetime and idempotent
cleanup. Return credentials explicitly for caller storage and renewal.

The transport package owns request construction, response decoding, and
transport error classification. It accepts an injected HTTPDoer and consumes
schema-generated wire structs from `pkg/dependencymodels`. The caller owns its HTTP transport configuration and
request deadlines. Add focused injection points for each other protocol the
client uses, including HTTP/2, WebSocket, MQTT, or RTC signaling. Keep
transport details out of public request and result types. Split generated model files and
their schemas by API responsibility, keeping a behavior API's request, response, and nested
objects together. Avoid a separate internal model bucket or handwritten duplicate wire
structs. Keep conversion and custom decoding behavior beside the related API component;
generate its field definitions. Verify public import paths from a separate consumer module.
