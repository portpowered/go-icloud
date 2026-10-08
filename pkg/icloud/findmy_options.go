package icloud

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	defaultFindMyMonitorInterval = 5 * time.Minute
	defaultFindMyFamilyDelay     = 500 * time.Millisecond
	defaultFindMyFamilyRetries   = 5
)

var errFindMyOptions = errors.New("invalid Find My polling configuration")

// FindMyScheduler supplies cancellable lifecycle waits without retaining account data.
// Wait must honor context cancellation. The session invokes it without holding state locks.
type FindMyScheduler interface {
	Wait(ctx context.Context, request FindMyWaitRequest) error
}

type findMySystemScheduler struct{}

func (findMySystemScheduler) Wait(ctx context.Context, request FindMyWaitRequest) error {
	timer := time.NewTimer(request.Delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for Find My lifecycle: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

type findMyConfiguration struct {
	scheduler       FindMyScheduler
	monitorInterval time.Duration
	familyDelay     time.Duration
	familyRetries   int
}

// FindMyOption configures one account-bound session's polling lifecycle.
type FindMyOption func(*findMyConfiguration) error

// WithFindMyMonitorInterval sets the background refresh interval; zero disables automatic refresh.
// The default is five minutes. Automatic refresh does not request updated device locations.
func WithFindMyMonitorInterval(interval time.Duration) FindMyOption {
	return func(config *findMyConfiguration) error {
		if interval < 0 {
			return errFindMyOptions
		}

		config.monitorInterval = interval

		return nil
	}
}

// WithFindMyFamilyPolling sets the readiness wait and maximum retries.
// Defaults are 500 milliseconds and five retries; polling stops early when readiness stalls.
func WithFindMyFamilyPolling(delay time.Duration, retries int) FindMyOption {
	return func(config *findMyConfiguration) error {
		if delay < 0 || retries < 0 {
			return errFindMyOptions
		}

		config.familyDelay, config.familyRetries = delay, retries

		return nil
	}
}

// WithFindMyScheduler injects lifecycle waits for offline replay or caller scheduling.
// A scheduler may be shared only when its implementation supports concurrent sessions.
func WithFindMyScheduler(scheduler FindMyScheduler) FindMyOption {
	return func(config *findMyConfiguration) error {
		if scheduler == nil {
			return errFindMyOptions
		}

		config.scheduler = scheduler

		return nil
	}
}

func findMyConfig(options []FindMyOption) (findMyConfiguration, error) {
	config := findMyConfiguration{scheduler: findMySystemScheduler{},
		monitorInterval: defaultFindMyMonitorInterval, familyDelay: defaultFindMyFamilyDelay,
		familyRetries: defaultFindMyFamilyRetries}

	for _, option := range options {
		if option == nil {
			return config, errFindMyOptions
		}

		err := option(&config)
		if err != nil {
			return config, fmt.Errorf("configure Find My session: %w", err)
		}
	}

	return config, nil
}
