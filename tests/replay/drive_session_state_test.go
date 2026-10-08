package replay_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/portpowered/go-icloud/internal/protocol"
	"github.com/portpowered/go-icloud/pkg/icloud"
	"github.com/portpowered/go-icloud/tests/replay"
)

type driveCookieCase struct {
	update   string
	next     string
	count    int
	explicit bool
}

const (
	nodeCookieHeaderKey   = "cookie"
	nodeSessionCookieName = "session"
	nodeSnapshotMutation  = "snapshot-mutated"
)

func TestDriveSessionCookieUpdatesAndDeletion(t *testing.T) {
	t.Parallel()

	cases := map[string]driveCookieCase{
		"uppercase-host-deletion": {update: "session=; Path=/; Max-Age=0", next: "other=second", count: 1, explicit: false},
		"case-insensitive-rotation": {
			update: "session=new; Domain=DRIVE.EXAMPLE.INVALID; Path=/; Secure; HttpOnly; SameSite=Lax",
			next:   "session=new; other=second", count: 2, explicit: false},
		"foreign-domain": {update: "session=foreign; Domain=foreign.invalid; Path=/",
			next: "session=old; other=second", count: 2, explicit: false},
		"explicit-header": {update: "session=new; Path=/", next: "explicit=fixed", count: 2, explicit: true},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkDriveSessionCookie(t, testCase)
		})
	}
}

func checkDriveSessionCookie(t *testing.T, testCase driveCookieCase) {
	t.Helper()
	session, transport, first := openDriveCookieControl(t, testCase)

	root, err := session.Root(t.Context(), icloud.DriveLocationRequest{Refresh: false})
	if err != nil {
		t.Fatal(err)
	}

	saved := session.Authentication()
	if len(saved.Cookies) != testCase.count {
		t.Fatal("deleted or foreign cookie survived the credential snapshot")
	}

	if testCase.next == "session=new; other=second" {
		checkDriveCookieAttributes(t, saved.Cookies[0])
	}

	saved.Cookies[0].Value = nodeSnapshotMutation

	checkDriveSnapshotOwnership(t, root)

	metadata := session.LastResponses()
	metadata[0].Headers[0].Value = "metadata-mutated"

	checkSDKMetadata(t, session.LastResponses()[0], first.Response)

	_, err = session.Root(t.Context(), icloud.DriveLocationRequest{Refresh: true})
	if err != nil {
		t.Fatal(err)
	}

	consumeErr := transport.AssertConsumed()
	if consumeErr != nil {
		t.Fatal(consumeErr)
	}
}

func checkDriveCookieAttributes(t *testing.T, cookie icloud.AuthCookie) {
	t.Helper()

	if !cookie.HTTPOnly || !cookie.Secure || cookie.SameSite == nil || *cookie.SameSite != icloud.AuthCookieSameSiteLax {
		t.Fatal("cookie persistence lost native attributes")
	}
}

func openDriveCookieControl(t *testing.T, testCase driveCookieCase) (
	*icloud.DriveSession, *replay.HTTPTransport, replay.Exchange,
) {
	t.Helper()
	scenario := readAccountScenario(t, "fixtures/synthetic/http/drive-root-listing-1.json")
	first := scenario.Exchanges[0]
	second := scenario.Exchanges[0]

	first.Request.Headers = append([]replay.Pair{}, first.Request.Headers...)
	second.Request.Headers = append([]replay.Pair{}, second.Request.Headers...)

	initial := "session=old; other=second"
	if testCase.explicit {
		initial = "explicit=fixed"
	}

	first.Request.Headers = append(first.Request.Headers, replay.Pair{nodeCookieHeaderKey, initial})
	second.Request.Headers = append(second.Request.Headers, replay.Pair{nodeCookieHeaderKey, testCase.next})
	response := *first.Response
	response.Headers = append(append([]replay.Pair{}, response.Headers...),
		replay.Pair{accountCookieUpdateHeader, testCase.update})
	first.Response = &response

	transport, err := replay.NewHTTPTransport([]replay.Exchange{first, second})
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.DriveServiceURL = scenario.Initial.Origin
	cookie := new(icloud.AuthCookie)
	cookie.Name, cookie.Value, cookie.Domain = nodeSessionCookieName, "old", "DRIVE.EXAMPLE.INVALID"
	cookie.Path, cookie.HostOnly = "/", true
	other := *cookie
	other.Name, other.Value = "other", "second"

	auth.Cookies = []icloud.AuthCookie{*cookie, other}
	if testCase.explicit {
		auth.Headers = append(auth.Headers, icloud.Header{Name: protocol.CookieName, Value: initial})
	}

	session, err := client.OpenDriveSession(t.Context(), icloud.OpenDriveSessionRequest{Auth: auth})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeErr := session.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	})

	auth.Cookies[0].Value = "caller-mutated"

	return session, transport, first
}

func checkDriveSnapshotOwnership(t *testing.T, root *icloud.DriveEntry) {
	t.Helper()

	value, err := root.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	want := value.Name
	(*value.Data.Items)[0].Name = &want
	*value.Data.Drivewsid = nodeSnapshotMutation

	value, err = root.Snapshot()

	if err != nil || value.Name != want || *value.Data.Drivewsid == nodeSnapshotMutation ||
		*(*value.Data.Items)[0].Name == want {
		t.Fatal("entry snapshot mutated cached node state", err)
	}
}

var errDriveSeek = errors.New("synthetic seek failure")

type driveFailingReader struct{}

func (driveFailingReader) Read(_ []byte) (int, error)         { return 0, errDriveSeek }
func (driveFailingReader) Seek(_ int64, _ int) (int64, error) { return 0, errDriveSeek }

func TestDriveSessionLocalFailureKeepsPriorResponses(t *testing.T) {
	t.Parallel()
	scenario := readAccountScenario(t, "fixtures/synthetic/http/drive-root-listing-1.json")

	transport, err := replay.NewHTTPTransport(scenario.Exchanges[:1])
	if err != nil {
		t.Fatal(err)
	}

	client, err := icloud.New(icloud.WithHTTPTransport(transport))
	if err != nil {
		t.Fatal(err)
	}

	auth := sdkAccountAuth(scenario.Initial)
	auth.DriveServiceURL = scenario.Initial.Origin
	auth.DriveDocumentServiceURL = scenario.Initial.DocumentOrigin

	session, err := client.OpenDriveSession(t.Context(), icloud.OpenDriveSessionRequest{Auth: auth})
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		closeErr := session.Close()
		if closeErr != nil {
			t.Error(closeErr)
		}
	}()

	root, err := session.Root(t.Context(), icloud.DriveLocationRequest{Refresh: false})
	if err != nil {
		t.Fatal(err)
	}

	before := session.LastResponses()
	_, err = root.Upload(t.Context(), icloud.DriveUploadRequest{Content: driveFailingReader{}, Filename: "synthetic.txt",
		CreationTime: nil, ModificationTime: nil})

	var failure *icloud.ClientError

	if !errors.As(err, &failure) || !errors.Is(err, errDriveSeek) || !reflect.DeepEqual(before, session.LastResponses()) {
		t.Fatal("local failure lost its cause or previous response evidence", err)
	}

	consumeErr := transport.AssertConsumed()
	if consumeErr != nil {
		t.Fatal(consumeErr)
	}
}
