# Synthetic SRP proof vectors

`python-vectors.json` contains synthetic passwords and account names, not captured
credentials. The expected public ephemeral and proofs were produced offline by
the pinned `pyicloud/srp_password.py` and installed `srp._pysrp` implementation:
RFC 5054 compatibility enabled, username omitted from the private-key derivation,
SHA-256, and the 2048-bit RFC group. The username still contributes to the proof.

The injected entropy is the byte sequence 0 through 255. The implementation sets
its leading high bit exactly as Python's `get_random_of_length(256)` does. The
server uses a synthetic verifier with the 256-byte secret consisting of repeated
`193` bytes. Both `s2k` and `s2k_fo` use the recorded salt and 1000 PBKDF2 rounds.
The latter stretches the lowercase hexadecimal SHA-256 password digest; the
former stretches the binary digest. The synthetic password includes a Unicode
character to check UTF-8 hashing.

The vectors compare complete public ephemeral, client proof, and server proof,
including the source's minimal-width proof components and padded scrambling and
multiplier hashes. Negative tests reject invalid public values, stretching
parameters, protocols, and entropy failures.
