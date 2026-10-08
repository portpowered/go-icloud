package icloud

import "context"

func beginSessionCall(ctx, lifetime context.Context, gate chan struct{},
	operation string, closedCause error,
) (context.Context, func(), error) {
	select {
	case <-lifetime.Done():
		return nil, nil, newClientError(operation, Closed, 0, nil, nil, closedCause)
	default:
	}

	select {
	case <-ctx.Done():
		return nil, nil, driveContextFailure(operation, ctx.Err())
	case <-lifetime.Done():
		return nil, nil, newClientError(operation, Closed, 0, nil, nil, closedCause)
	case <-gate:
	}

	if lifetime.Err() != nil {
		gate <- struct{}{}

		return nil, nil, newClientError(operation, Closed, 0, nil, nil, closedCause)
	}

	if ctx.Err() != nil {
		gate <- struct{}{}

		return nil, nil, driveContextFailure(operation, ctx.Err())
	}

	call, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(lifetime, cancel)
	finish := func() {
		stop()
		cancel()

		gate <- struct{}{}
	}

	return call, finish, nil
}
