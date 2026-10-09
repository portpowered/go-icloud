// Package bridge implements the trusted-device socket protocol.
package bridge

import "fmt"

// ProtocolError records the protocol stage without disclosing challenge secrets.
type ProtocolError struct {
	Stage string
	Cause error
}

func (failure *ProtocolError) Error() string {
	return fmt.Sprintf("trusted device bridge %s: %v", failure.Stage, failure.Cause)
}

func (failure *ProtocolError) Unwrap() error { return failure.Cause }

// InvalidNonceError requests one bootstrap retry using the server's millisecond clock.
type InvalidNonceError struct{ TimestampMilliseconds uint64 }

func (failure *InvalidNonceError) Error() string { return "bridge rejected bootstrap nonce" }
