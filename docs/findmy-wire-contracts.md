# Find My wire contracts

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
`pyicloud/services/findmyiphone.py`. Thirty-seven implementation-derived synthetic
scenarios contain 70 paired exchanges. Contract checks bind every exchange to
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

All 65 JSON replies round-trip through their canonical generated models.
Four empty 204 command acknowledgements and one binary 201 acknowledgement
remain uninterpreted. A sixth case exercises a JSON 201 acknowledgement with
unknown metadata; all six new cases execute the actual pinned Source.
Command routes declare 204 no-content and successful 2XX ownership, separately
from default provider failures. Their bytes remain bound by strict paired replay. Exact declared response media
precedes wildcard binary fallback; 201/202 JSON ownership controls require the
explicit acknowledgement schema and reject non-object JSON. Focused
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
and location controls. Full session orchestration still requires the SDK adapter.

Refresh content is optional. Missing or empty content is distinct from a claim
that cached devices were removed. Family members use an open fetch-status string;
LOADING members drive bounded polling, while absent feature flags do not prove
capabilities. Provider server context remains extensible and opaque except for
the reference's treatment of `theftLoss`. Unknown metadata is retained as raw
JSON. Device identifiers are required where the Source indexes them; other
device metadata remains optional (SCHEMA-04, SCHEMA-07).

The erase-token reply can represent missing tokens so that the adapter can stop
before an erase command. Sound, message, lost mode and erase acknowledgements
remain extensible and do not establish completed device action. The eventual
adapter must return typed refusal evidence, preserve account isolation and avoid
automatic retries of uncertain commands (API-11, API-14).

## Remaining acceptance

The [public Find My session](findmy-session.md) now executes all 37 current
portable scenarios and 70 pairs through the SDK with semantic projection checks.
The schema/model checks remain a separate layer. Selected Source status/location
and timed monitor behavior, the full source/model/runtime binding gate, live Go
integration, native Go authentication/forced reauth and release acceptance remain
open. Synthetic commands are offline only; live authorization covers reads.
