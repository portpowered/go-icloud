# HTTP generation templates

These templates are derived from oapi-codegen v2.8.0 (Copyright 2019 DeepMap, Inc.) under Apache-2.0; its license is included beside them. The modifications retain upstream request constructors, response models, and response parsers while removing generated clients, transport defaults, request editor callbacks, and HTTP send methods.

SCHEMA-10 and GO-13 require reproducible schema-owned wire artifacts. LIB-01 keeps network injection, origin validation, request editing, body ownership, and cancellation in `pkg/dependencies/webtransport`, which consumes the generated constructors. The removed convenience clients were newly introduced dependency implementation types; public SDK operations and their paired replay coverage remain supported. Regeneration uses the pinned generator through `make generate`, and the complete wire audit still inspects generated sources for any outbound call introduced later.
