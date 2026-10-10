# SDK maintenance boundaries

`pkg/icloud` is the public SDK. Its named request/result projections are generated
from `api/client-models.openapi.yaml` and are independent of provider wire structs
(API-01–API-03). Keep the operation inventory together in the Client interface.

Provider contracts live in responsibility-specific external schemas and generate
wire types under `pkg/dependencymodels`. Transport behavior belongs under
`pkg/dependencies/<transport>` (template item 7). HTTP behavior and generated
service route clients live under `pkg/dependencies/webtransport`; socket framing,
bridge connections and hardware authentication have separate dependency packages.
Companion code may convert or decode generated fields; it must not redefine wire
models, fixed values or nested library-owned payload keys.

The reusable SDK stores immutable dependencies, never account credentials. Each
request supplies authentication. An explicit Drive, Find My or authorization session
owns state for one account, exposes updated credentials, accepts cancellation and
closes idempotently. Return token rotation explicitly for caller storage
(API-04, API-13; template items 9 and 11).

Injected dependencies must be safe for concurrent use. HTTP transports may not
transfer cookies between accounts. Every non-HTTP edge needs an offline
connection-producing injection point, not merely a configurable concrete dialer.
Test full requests from two accounts through one reusable client.

Typed errors preserve causes and exact response evidence while avoiding provider
secrets in their printable text. Response metadata includes scoped cookie updates;
multistep failures keep prior responses. A successful acknowledgement does not
prove completed background or physical behavior. Do not automatically retry an
uncertain write (API-11, API-14).

See [customer configuration](guides/configuration.mdx), [verification](verification.md),
and the [library standards](library-standards.md).
