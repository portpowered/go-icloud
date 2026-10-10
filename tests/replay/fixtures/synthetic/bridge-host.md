# Bridge hostname examples

The 55 cases in `bridge-host.json` execute `_resolve_websocket_host` extracted
from the pinned Python source with invented bootstrap objects. They are offline
implementation-derived examples, not provider captures. Run
`python tools/reference/bridge_host_fixtures.py --source PATH_TO_PINNED_CHECKOUT`
to regenerate the exact outputs. No Python interpreter participates in SDK use.

The paired cases cover URL versus bare-host case, user information and port,
IPv6 addresses, raw and escaped zone suffix spelling and case, Unicode lowercase
including dotted I and contextual final sigma, percent spelling, IPvFuture,
empty authority fallback, both named environments, precedence and missing hosts.

Further paired cases cover invalid port text/range, control preprocessing,
malformed and custom schemes, authority delimiters and multiple user information
segments, malformed bracketed IP/IPvFuture addresses and zones, and NFKC authority
validation. Every generated parser failure retains its exact Python exception
type and message; Go verifies the parser message and wrapped cause. The SDK uses
the pinned `x/text` Unicode lowercase and normalization implementation (GO-06).
The fixed case inventory establishes these semantics rather than claiming
exhaustive coverage of every Unicode code point or arbitrary input (LIB-18).
