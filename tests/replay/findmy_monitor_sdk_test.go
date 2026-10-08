package replay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

const findMyMonitorReplayDeadline = 10 * time.Second

var errFindMyMonitorTrace = errors.New("invalid monitor replay trace")

type findMyMonitorWait struct {
	DelaySeconds    float64 `json:"delaySeconds"`
	CompletedAtUnix float64 `json:"completedAtUnix"`
	Stopped         bool    `json:"stopped"`
}

type findMyMonitorInput struct {
	IntervalSeconds float64 `json:"intervalSeconds"`
}

type findMyMonitorOutcome struct {
	State      findMySDKState        `json:"state"`
	Failed     bool                  `json:"failed"`
	HTTPStatus int                   `json:"httpStatus"`
	Cookies    []findMyMonitorCookie `json:"cookies"`
}

type findMyMonitorCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HostOnly bool   `json:"hostOnly"`
}

type findMyMonitorScheduler struct {
	waits   chan icloud.FindMyWaitRequest
	permits chan struct{}
}

func (scheduler *findMyMonitorScheduler) Wait(ctx context.Context, request icloud.FindMyWaitRequest) error {
	select {
	case scheduler.waits <- request:
	case <-ctx.Done():
		return fmt.Errorf("monitor replay canceled before wait: %w", ctx.Err())
	}

	select {
	case <-scheduler.permits:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("monitor replay canceled during wait: %w", ctx.Err())
	}
}

func runFindMyMonitorSDK(t *testing.T, scenario findMySDKScenario) {
	t.Helper()

	traceErr := validateFindMyMonitorTrace(scenario.Entropy.Monitor, scenario.Entropy.UnixSeconds)
	if traceErr != nil {
		t.Fatal(traceErr)
	}

	ctx, cancel := context.WithTimeout(t.Context(), findMyMonitorReplayDeadline)
	defer cancel()

	transport, err := replay.NewHTTPTransport(scenario.Exchanges)
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	var input findMyMonitorInput

	err = json.Unmarshal(scenario.Inputs[0], &input)
	if err != nil {
		t.Fatal(err)
	}

	scheduler := &findMyMonitorScheduler{waits: make(chan icloud.FindMyWaitRequest), permits: make(chan struct{})}

	session, err := client.OpenFindMySession(ctx, icloud.OpenFindMySessionRequest{
		Auth: findMySDKAuth(t, scenario.Initial), IncludeFamily: scenario.Initial.Family},
		icloud.WithFindMyMonitorInterval(time.Duration(input.IntervalSeconds*float64(time.Second))),
		icloud.WithFindMyScheduler(scheduler))
	if err != nil {
		t.Fatal(err)
	}

	defer closeFindMyReplay(t, session)

	outcomes := driveFindMyMonitorReplay(ctx, t, scenario, session, scheduler)
	assertFindMySDKJSON(t, outcomes, scenario.Result)

	err = transport.AssertConsumed()
	if err != nil {
		t.Fatal(err)
	}
}

func driveFindMyMonitorReplay(ctx context.Context, t *testing.T, scenario findMySDKScenario,
	session *icloud.FindMySession, scheduler *findMyMonitorScheduler,
) []findMyMonitorOutcome {
	t.Helper()

	values := make([]findMyMonitorOutcome, 0, len(scenario.Entropy.Monitor))
	cursor := 0
	completed := scenario.Entropy.UnixSeconds

	for index, expected := range scenario.Entropy.Monitor {
		completed = checkFindMyMonitorCompletion(t, expected, completed)

		var wait icloud.FindMyWaitRequest
		select {
		case wait = <-scheduler.waits:
		case <-ctx.Done():
			t.Fatal("Find My monitor stopped making replay progress")
		}

		if wait.Kind != icloud.FindMyMonitorWait || wait.Delay.Seconds() != expected.DelaySeconds {
			t.Fatal("Find My monitor wait kind or duration changed")
		}

		cursor = checkFindMySDKResponses(t, scenario, cursor, session.LastResponses())

		values = append(values, findMyMonitorResult(t, scenario, session, cursor))

		if expected.Stopped {
			stopFindMyReplayMonitor(ctx, t, scenario, session, index)

			break
		}

		select {
		case scheduler.permits <- struct{}{}:
		case <-ctx.Done():
			t.Fatal("Find My monitor did not consume its completed wait")
		}
	}

	if cursor != len(scenario.Exchanges) {
		t.Fatal("Find My monitor left unconsumed response evidence")
	}

	return values
}

func checkFindMyMonitorCompletion(t *testing.T, expected findMyMonitorWait, previous float64) float64 {
	t.Helper()

	if expected.CompletedAtUnix < previous ||
		(!expected.Stopped && expected.CompletedAtUnix <= previous+expected.DelaySeconds) {
		t.Fatal("Find My monitor completion did not advance beyond the declared interval")
	}

	return expected.CompletedAtUnix
}

func stopFindMyReplayMonitor(ctx context.Context, t *testing.T, scenario findMySDKScenario,
	session *icloud.FindMySession, index int,
) {
	t.Helper()

	if index != len(scenario.Entropy.Monitor)-1 {
		t.Fatal("unconsumed monitor completion events")
	}

	closeFindMyReplay(t, session)
	awaitFindMyReplayMonitor(ctx, t, session)
}

func findMyMonitorResult(t *testing.T, scenario findMySDKScenario, session *icloud.FindMySession,
	cursor int,
) findMyMonitorOutcome {
	t.Helper()

	result := findMyMonitorOutcome{State: findMySDKSnapshot(t, session), Failed: false, HTTPStatus: 0,
		Cookies: make([]findMyMonitorCookie, 0)}
	for _, cookie := range session.Authentication().Cookies {
		result.Cookies = append(result.Cookies, findMyMonitorCookie{Name: cookie.Name, Value: cookie.Value,
			Domain: cookie.Domain, Path: cookie.Path, Secure: cookie.Secure, HostOnly: cookie.HostOnly})
	}

	failure := session.LastError()
	if failure == nil {
		return result
	}

	if failure.Kind() != icloud.Unavailable ||
		!bytes.Equal(failure.ResponseBody(), findMyResponseBytes(t, scenario.Exchanges[cursor-1].Response.Body)) {
		t.Fatal("monitor failure lost typed meaning or exact provider body")
	}

	result.Failed, result.HTTPStatus = true, failure.StatusCode()

	return result
}

func closeFindMyReplay(t *testing.T, session *icloud.FindMySession) {
	t.Helper()

	err := session.Close()
	if err != nil {
		t.Error(err)
	}
}

func awaitFindMyReplayMonitor(ctx context.Context, t *testing.T, session *icloud.FindMySession) {
	t.Helper()

	select {
	case <-session.MonitorDone():
	case <-ctx.Done():
		t.Fatal("Find My monitor did not exit after close")
	}
}

func validateFindMyMonitorTrace(events []findMyMonitorWait, initial float64) error {
	if len(events) == 0 || !events[len(events)-1].Stopped || math.IsInf(initial, 0) || math.IsNaN(initial) {
		return errFindMyMonitorTrace
	}

	for _, event := range events[:len(events)-1] {
		if event.Stopped {
			return errFindMyMonitorTrace
		}
	}

	for _, event := range events {
		if !validFindMyMonitorTimes(event) {
			return errFindMyMonitorTrace
		}
	}

	return nil
}
func validFindMyMonitorTimes(event findMyMonitorWait) bool {
	return event.DelaySeconds > 0 && !math.IsInf(event.DelaySeconds, 0) && !math.IsNaN(event.DelaySeconds) &&
		!math.IsInf(event.CompletedAtUnix, 0) && !math.IsNaN(event.CompletedAtUnix)
}
func TestFindMyMonitorTraceRequiresFiniteTerminalStop(t *testing.T) {
	t.Parallel()

	for _, trace := range [][]findMyMonitorWait{
		nil,
		{{DelaySeconds: 60, CompletedAtUnix: 1061, Stopped: false}},
		{{DelaySeconds: 60, CompletedAtUnix: 1061, Stopped: true},
			{DelaySeconds: 60, CompletedAtUnix: 1061, Stopped: true}},
		{{DelaySeconds: 60, CompletedAtUnix: math.Inf(1), Stopped: true}},
		{{DelaySeconds: math.NaN(), CompletedAtUnix: 1061, Stopped: true}},
	} {
		err := validateFindMyMonitorTrace(trace, 1000)
		if !errors.Is(err, errFindMyMonitorTrace) {
			t.Fatal("monitor accepted an unbounded or unterminated artifact trace")
		}
	}
}
