package icloud

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var errAuthenticationWaiter = errors.New("authentication wait function is required")

// AuthenticationWaiter waits for an authentication polling interval and honors cancellation.
// Injected implementations must be safe for concurrent accounts and return context cancellation.
type AuthenticationWaiter func(context.Context, time.Duration) error

// WithAuthenticationWait injects the wait used by native authentication polling.
func WithAuthenticationWait(wait AuthenticationWaiter) Option {
	return func(config *configuration) error {
		if wait == nil {
			return errAuthenticationWaiter
		}

		config.authWait = wait

		return nil
	}
}

func authenticationWait(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	err := ctx.Err()
	if err != nil {
		return fmt.Errorf("authentication polling wait: %w", err)
	}
	return nil
}
