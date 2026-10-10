# Bridge hostname examples

The 19 cases in `bridge-host.json` execute `_resolve_websocket_host` extracted
from the pinned Python source with invented bootstrap objects. They are offline
implementation-derived examples, not provider captures. Run
`python tools/reference/bridge_host_fixtures.py --source PATH_TO_PINNED_CHECKOUT`
to regenerate the exact outputs. No Python interpreter participates in SDK use.

The paired cases cover URL versus bare-host case, user information and port,
IPv6 addresses, raw and escaped zone suffix spelling and case, Unicode lowercase
including dotted I and contextual final sigma, percent spelling, IPvFuture,
empty authority fallback, both named environments, precedence and missing hosts.

The Go URL parser still differs from Python's parser outside this case inventory:
Python permits invalid port text when only `hostname` is read, removes embedded
ASCII tabs/newlines before parsing, and falls back for malformed scheme text.
Python rejects malformed bracketed IP literals and certain NFKC-changing authority
characters that Go accepts. These differences require separate parser work before
claiming complete arbitrary-input hostname conformance (LIB-18).
