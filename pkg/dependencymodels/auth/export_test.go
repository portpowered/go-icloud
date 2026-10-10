package auth

import "encoding/json"

// CorruptAuthLoginRequest exposes malformed private storage for codec controls.
func CorruptAuthLoginRequest(data []byte) AuthLoginRequest {
	return AuthLoginRequest{union: json.RawMessage(data)}
}
