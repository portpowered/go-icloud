// Package bridge implements the trusted-device socket protocol.
package bridge

import "fmt"

// Failure records the protocol stage without disclosing challenge secrets.
type Failure struct {
	Stage string
	Cause error
}

func (failure *Failure) Error() string {
	return fmt.Sprintf("trusted device bridge %s: %v", failure.Stage, failure.Cause)
}

func (failure *Failure) Unwrap() error { return failure.Cause }

// InvalidNonce requests one bootstrap retry using the server's millisecond clock.
type InvalidNonce struct{ TimestampMilliseconds uint64 }

func (failure *InvalidNonce) Error() string { return "bridge rejected bootstrap nonce" }
