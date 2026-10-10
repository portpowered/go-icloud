# Bridge hostname examples

The 62 cases in `bridge-host.json` execute `_resolve_websocket_host` extracted
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
Source-derived Unicode15.0.0 lowercase/context tables and pinned `x/text` NFKC
normalization implementation (GO-06). The final sigma context scan is unbounded,
including runs of dots and combining characters longer than 30 characters.
The fixed case inventory establishes these semantics rather than claiming
exhaustive coverage of every Unicode code point or arbitrary input (LIB-18).

`python tools/reference/generate_host_unicode.py` regenerates the private
lowercase and sigma context tables using Python's Unicode15.0.0 `str.lower`.
All Unicode scalar values are enumerated. Two sigma context probes derive the
effective case-ignorable/cased classification; generated ranges are ordered and
disjoint. Runtime code handles the context scan and zone preservation separately
from generated data. `python tools/reference/generate_host_unicode.py --check`
fails on version or generation drift. The AsyncAPI hostname profile points to
`api/dependency-profiles/bridge-host-unicode.json`, which records the source
version, generator, table denominators and SHA256 of the formatted generated
file. Its hash verifies generated dependency data, not executable functions.
