# Find My wire contracts

Contributor contract history: measured counts and remaining-work statements below
refer to their individual milestones. Current acceptance is maintained in
[completion matrix](completion-matrix.md); customer usage belongs in MDX guides.


`api/external/findmy.openapi.yaml` describes seven Find My HTTP operations:
device initialization and refresh, sound, messaging, lost mode, erase-token
lookup and remote erase. Authenticated discovery supplies the caller's Find My
and setup origins. Token lookup uses `/setup/ws/1/fmipWebAuthenticate` on the
setup origin and omits the ordinary account query parameters.

`api/external/findmy-models.openapi.yaml` owns canonical models and generates
`pkg/dependencymodels/findmy`. The internal generated client imports those models.
Protocol paths, methods, parameter names, fields and fixed string values come
from these schema documents. `make generate-api` regenerates them; contract and
generator checks reject drift and ambiguous operation/model constants
(SCHEMA-10, SCHEMA-16).

## Evidence and validation

The source is timlaing/pyicloud commit
`e2e44ab875d47dab4475096021da60030f26c35e`, particularly
`pyicloud/services/findmyiphone.py`. Seventy-one implementation-derived synthetic
scenarios contain 140 paired exchanges. Contract checks bind every exchange to
one of the seven routes and validate request and response payloads, media and
required account query values. Controls reject unknown routes/methods, missing,
unknown or repeated query values, and account parameters on token lookup.
The existing paired matcher verifies complete wire bytes, order and consumption;
contract validation does not replace it (LIB-05).

Known device fields and nested metadata also follow the pinned Source test
constants. Those constants are historical reference examples, not captured Apple
traffic and not additional portable SDK replays. The four schema examples are
invented. No private capture, account data, credential or live device command is
published or performed (SCHEMA-08, SCHEMA-12).

All 127 valid JSON replies round-trip through their canonical generated models.
Command reply bytes have an opaque body contract on both explicit 200 and 2XX
responses, regardless of advertised media. The actual Source session tolerates
malformed JSON and the command methods do not parse their replies. A separate
FindMyAcknowledgement model preserves the optional parsed view of any valid JSON
object, array, scalar or null without floating-point conversion. This removes
an unsupported object-only assumption (SCHEMA-04, SCHEMA-07).

Four empty 204 acknowledgements, five binary acknowledgements and four malformed
JSON acknowledgements are bound byte-for-byte rather than parsed. Eight additional
cases prove that recognized application/json or text/json provider error objects
at HTTP 200 are failures, with exact response context. All four command methods
have binary-200, malformed-JSON-200, array-202, scalar-200 and both provider-error
media cases. No body matcher is weakened: requests, response bytes, metadata,
order and full consumption remain mandatory (LIB-05). The byte contract does not
allow error objects to be acknowledged as successful physical actions.

Focused
controls retain unknown large integers, nulls and arrays, omitted versus explicit
null locations/command state, empty content and future feature flags. Latitude,
longitude, altitude, accuracy and battery level use 64-bit numbers; a regression
binds nontrivial decimal values to catch precision loss during generation.
Timestamp values retain their recorded integer units and representation.

## Request and state semantics

Initialization sends the fixed application/version/timezone context and the
selected family flag. It omits provider server context and location-update flags.
Refresh includes provider context; location updates add `shouldLocate`,
`selectedDevice` and `isUpdatingAllLocations`. Separate initialization and refresh
models prevent a setup request from silently accepting refresh state. Schema
field order follows the pinned request encoding. The [internal transport](findmy-transport.md) applies Source-style JSON encoding
and location controls. The public SDK session executes the same portable artifacts and owns orchestration.

Refresh content is optional. Missing or empty content is distinct from a claim
that cached devices were removed. Family members use an open fetch-status string;
LOADING members drive bounded polling, while absent feature flags do not prove
capabilities. Provider server context remains extensible and opaque except for
the reference's treatment of `theftLoss`. Unknown metadata is retained as raw
JSON. Device identifiers are required where the Source indexes them; other
device metadata remains optional (SCHEMA-04, SCHEMA-07).

The erase-token reply can represent missing tokens so that the adapter can stop
before an erase command. Sound, message, lost mode and erase acknowledgements
remain opaque and do not establish completed device action. The SDK returns
typed refusal evidence, preserves account isolation and sends each command once
without automatic retries (API-11, API-14).

## Remaining acceptance

The [public Find My session](findmy-session.md) now executes all 71 current
portable scenarios and 140 pairs through the SDK with semantic projection checks.
The schema/model checks remain a separate layer. Remaining Source implicit getter refresh, integer selection and timer boundary/skew
behavior, the full source/model/runtime binding gate, live Go
integration, native Go authentication/forced reauth and release acceptance remain
open. Synthetic commands are offline only; live authorization covers reads.

Generated command response parsers also preserve raw Body bytes for all media
and statuses. Their wildcard byte schemas prevent oapi-codegen from decoding
advertised JSON into []byte or routing successful replies through a default
JSON error parser. A contract test executes all 38 command reply artifacts
through the four generated parsers and requires exact byte equality. The shared
SDK raw reader still owns provider error classification and typed public errors;
canonical valid-JSON projections remain separately checked. Generation drift
checks bind this behavior to the checked-in schema (SCHEMA-10, SCHEMA-16).
