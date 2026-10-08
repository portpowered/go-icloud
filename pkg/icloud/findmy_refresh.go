package icloud

import (
	"context"
	"errors"
	"maps"

	"github.com/portpowered/go-icloud/internal/webtransport"
	"github.com/portpowered/go-icloud/pkg/dependencymodels/findmy"
)

// Refresh fetches devices, performs bounded family readiness polling and returns a copied cache.
// Empty partial replies retain cached devices; updated records replace whole device records.
func (session *FindMySession) Refresh(ctx context.Context, request RefreshFindMyRequest) (*FindMySnapshot, error) {
	return session.refresh(ctx, "FindMyRefresh", request.Locate, true)
}

func (session *FindMySession) refresh(ctx context.Context, operation string,
	locate, pollFamily bool,
) (*FindMySnapshot, error) {
	call, finish, err := session.begin(ctx, operation)
	if err != nil {
		return nil, err
	}
	defer finish()

	session.clearResponses()

	err = session.refreshStep(call, operation, locate)
	if err == nil && pollFamily {
		err = session.pollFamily(call, operation, locate)
	}

	if err == nil && pollFamily {
		session.mu.Lock()
		empty := len(session.devices) == 0
		session.mu.Unlock()

		if empty {
			err = newClientError(operation, NoDevices, 0, nil, nil, errFindMyNoDevices)
		}
	}

	if err != nil {
		failure := session.recordFailure(operation, err)

		return nil, failure
	}

	session.clearLastError()

	return session.Snapshot()
}

func (session *FindMySession) refreshStep(ctx context.Context, operation string, locate bool) error {
	auth := session.Authentication()

	boundary, err := findMyRequestContext(auth)
	if err != nil {
		return newClientError(operation, Configuration, 0, nil, nil, err)
	}

	session.mu.Lock()
	server := append(findmy.FindMyRefreshContext(nil), session.server...)
	session.mu.Unlock()

	var response *webtransport.FindMyDevicesResponse
	if webtransport.HasFindMyContext(server) {
		response, err = session.client.web.RefreshFindMy(ctx, boundary, server, session.family, locate)
	} else {
		response, err = session.client.web.InitializeFindMy(ctx, boundary, session.family)
	}

	if err != nil {
		return adaptFailure(operation, err)
	}

	session.recordResponse(publicMetadata(response.Response))

	server, err = webtransport.NormalizeFindMyContext(response.Context)
	if err != nil {
		return newClientError(operation, InvalidResponse, 0, nil, nil, err)
	}

	if ctx.Err() != nil {
		return driveContextFailure(operation, ctx.Err())
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	session.server, session.user = server, response.Data.UserInfo

	if response.Data.Content != nil {
		for _, device := range *response.Data.Content {
			if _, exists := session.devices[device.Id]; !exists {
				session.order = append(session.order, device.Id)
			}

			session.devices[device.Id] = device
		}
	}

	return nil
}

func (session *FindMySession) pollFamily(ctx context.Context, operation string, locate bool) error {
	previous := make(map[string]struct{})

	for range session.config.familyRetries {
		loading := session.loadingMembers()
		if len(loading) == 0 || maps.Equal(previous, loading) {
			break
		}

		previous = loading

		err := session.config.scheduler.Wait(ctx, FindMyWaitRequest{Kind: FindMyFamilyPollWait,
			Delay: session.config.familyDelay})
		if err != nil {
			return findMyWaitFailure(operation, err)
		}

		err = session.refreshStep(ctx, operation, locate)
		if err != nil {
			return err
		}
	}

	return nil
}

func (session *FindMySession) loadingMembers() map[string]struct{} {
	session.mu.Lock()
	defer session.mu.Unlock()

	result := make(map[string]struct{})
	if !session.family || !session.user.IsSpecified() || session.user.IsNull() {
		return result
	}

	user := session.user.MustGet()
	if user.HasMembers == nil || !*user.HasMembers || user.MembersInfo == nil {
		return result
	}

	for key, member := range *user.MembersInfo {
		if member.DeviceFetchStatus != nil && *member.DeviceFetchStatus == string(findmy.LOADING) {
			result[key] = struct{}{}
		}
	}

	return result
}

func (session *FindMySession) monitor() {
	defer close(session.monitorDone)

	for {
		err := session.config.scheduler.Wait(session.lifetime,
			FindMyWaitRequest{Kind: FindMyMonitorWait, Delay: session.config.monitorInterval})
		if err != nil {
			if session.lifetime.Err() == nil {
				session.recordMonitorWaitFailure(err)
			}

			return
		}

		_, err = session.refresh(session.lifetime, "FindMyMonitor", false, false)
		if session.lifetime.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
	}
}

func (session *FindMySession) recordMonitorWaitFailure(err error) {
	const operation = "FindMyMonitorWait"

	_, finish, beginErr := session.begin(session.lifetime, operation)
	if beginErr != nil {
		return
	}

	defer finish()

	session.clearResponses()
	_ = session.recordFailure(operation, findMyWaitFailure(operation, err))
}

func findMyWaitFailure(operation string, err error) *ClientError {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return driveContextFailure(operation, err)
	}

	return newClientError(operation, Transport, 0, nil, nil, err)
}
