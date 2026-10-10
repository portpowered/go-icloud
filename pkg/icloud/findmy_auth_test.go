package icloud_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/portpowered/go-icloud/pkg/icloud"
)

const findMyCopyMutation = "copy-mutated"

func TestFindMySessionsOwnCredentialsCookiesAndSnapshots(t *testing.T) {
	t.Parallel()

	var callMu sync.Mutex

	calls := make(map[string]int)

	client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(request *http.Request) (*http.Response, error) {
		tenant := request.Header.Get("X-Tenant")

		callMu.Lock()
		calls[tenant]++
		count := calls[tenant]
		callMu.Unlock()

		cookie, cookieErr := request.Cookie("session")

		expected := tenant
		if count > 1 {
			expected += "-updated"
		}

		if cookieErr != nil || cookie.Value != expected {
			t.Error("Find My cookie crossed account boundaries or lost the provider update")
		}

		response := findMyReply(http.StatusOK, fmt.Sprintf(
			`{"content":[{"id":%q,"future":{"nested":[null,true]}}],"serverContext":{"token":"one"}}`, tenant))
		response.Header.Add("Set-Cookie", "session="+tenant+"-updated; Path=/; Secure")
		response.Header.Add("Set-Cookie", "foreign=refused; Domain=foreign.example.invalid; Path=/")

		return response, nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	for _, tenant := range []string{"alpha", "beta"} {
		t.Run(tenant, func(t *testing.T) {
			t.Parallel()
			checkFindMyTenant(t, client, tenant)
		})
	}
}

func checkFindMyTenant(t *testing.T, client *icloud.SDK, tenant string) {
	t.Helper()

	auth := findMyAuth()
	auth.Headers = append(auth.Headers, icloud.Header{Name: "X-Tenant", Value: tenant})
	cookie := new(icloud.AuthCookie)
	cookie.Name, cookie.Value, cookie.Secure = "session", tenant, true
	auth.Cookies = []icloud.AuthCookie{*cookie}

	session, err := client.OpenFindMySession(t.Context(), icloud.OpenFindMySessionRequest{
		Auth: auth, IncludeFamily: false}, icloud.WithFindMyMonitorInterval(0))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		closeErr := session.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	}()

	if auth.Cookies[0].Domain != "" || auth.Cookies[0].Value != tenant {
		t.Fatal("opening the session mutated caller credentials")
	}

	auth.Cookies[0].Value = testCallerMutation
	*auth.SessionToken = testCallerMutation

	copyAuth := session.Authentication()
	if *copyAuth.SessionToken != testSessionToken || len(copyAuth.Cookies) != 1 {
		t.Fatal("credential alias or foreign cookie escaped the session boundary")
	}

	*copyAuth.SessionToken = findMyCopyMutation
	copyAuth.Cookies[0].Value = findMyCopyMutation

	_, err = session.Refresh(t.Context(), icloud.RefreshFindMyRequest{Locate: false})
	if err != nil {
		t.Fatal(err)
	}

	checkFindMySnapshotCopies(t, session, tenant)
}

func checkFindMySnapshotCopies(t *testing.T, session *icloud.FindMySession, tenant string) {
	t.Helper()

	snapshot, err := session.Snapshot()
	if err != nil || snapshot.Devices[0].Id != tenant {
		t.Fatal("Find My account cache crossed session boundaries")
	}

	snapshot.Devices[0].AdditionalProperties["future"][0] = '!'
	snapshot.ServerContext.MustGet()[0] = '!'
	responses := session.LastResponses()
	responses[0].Headers[0].Value = findMyCopyMutation

	again, err := session.Snapshot()
	if err != nil || again.Devices[0].AdditionalProperties["future"][0] != '{' ||
		again.ServerContext.MustGet()[0] != '{' || session.LastResponses()[0].Headers[0].Value == findMyCopyMutation {
		t.Fatal("Find My results exposed mutable state")
	}
}

func TestFindMyOpeningFamilyWaitIsCancellable(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), findMyTestDeadline)
	defer cancel()

	waiting := make(chan struct{})
	scheduler := findMySchedulerFunc(func(call context.Context, request icloud.FindMyWaitRequest) error {
		if request.Kind != icloud.FindMyFamilyPollWait || request.Delay != 500*time.Millisecond {
			t.Error("opening requested the wrong family wait")
		}

		close(waiting)
		<-call.Done()

		return fmt.Errorf("cancel family wait: %w", call.Err())
	})

	client, err := icloud.New(icloud.WithHTTPTransport(sdkRoundTrip(func(_ *http.Request) (*http.Response, error) {
		return findMyReply(http.StatusOK, `{"content":[{"id":"device"}],"serverContext":{"token":"one"},`+
			`"userInfo":{"hasMembers":true,"membersInfo":{"member":{"deviceFetchStatus":"LOADING"}}}}`), nil
	})))
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)

	go func() {
		_, openErr := client.OpenFindMySession(ctx, icloud.OpenFindMySessionRequest{
			Auth: findMyAuth(), IncludeFamily: true}, icloud.WithFindMyScheduler(scheduler))
		done <- openErr
	}()

	awaitFindMySignal(ctx, t, waiting)
	cancel()
	assertSDKKind(t, <-done, icloud.Canceled)
}
