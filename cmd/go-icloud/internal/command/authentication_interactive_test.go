package command_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/portpowered/go-icloud/cmd/go-icloud/internal/command"
	"github.com/portpowered/go-icloud/pkg/icloud"
)

const interactiveClientID = "synthetic-client"

type interactiveAuthProbe struct {
	icloud.Client

	login         icloud.AuthenticateRequest
	verification  icloud.VerifyTwoFactorCodeRequest
	pendingSMS    bool
	selectedPhone *icloud.TrustedPhoneNumberID
}

func (p *interactiveAuthProbe) Authenticate(_ context.Context, req icloud.AuthenticateRequest,
) (*icloud.NativeAuthResult, error) {
	p.login = req

	var result icloud.NativeAuthResult

	result.State.Auth.ClientID = interactiveClientID
	result.State.RequiresMFA, result.State.CodeRequested = true, true
	result.State.DeliveryMethod = icloud.TwoFactorDeliverySMS

	if p.pendingSMS {
		var voice, sms icloud.TrustedPhoneNumberID
		err := voice.FromTrustedPhoneNumberID0(1)
		if err != nil {
			return nil, fmt.Errorf("encode synthetic phone: %w", err)
		}

		err = sms.FromTrustedPhoneNumberID0(2)
		if err != nil {
			return nil, fmt.Errorf("encode synthetic phone: %w", err)
		}

		var first, second icloud.TrustedPhoneNumber

		first.ID, first.PushMode, second.ID, second.PushMode = voice, "voice", sms, "sms"
		result.State.Challenge.PhoneNumbers = []icloud.TrustedPhoneNumber{first, second}
		result.State.CodeRequested = false
	}

	return &result, nil
}

func (p *interactiveAuthProbe) RequestTwoFactorCode(_ context.Context, req icloud.RequestTwoFactorCodeRequest,
) (*icloud.NativeAuthResult, error) {
	p.selectedPhone = req.PhoneNumberID

	var result icloud.NativeAuthResult

	result.State, result.Success = req.State, true
	result.State.CodeRequested = true

	return &result, nil
}

func TestAuthLoginSelectsAnSMSCapablePhone(t *testing.T) {
	t.Parallel()

	var probe interactiveAuthProbe

	probe.pendingSMS = true

	var output, diagnostic bytes.Buffer

	err := command.RunWithInput(t.Context(), &probe,
		[]string{"auth", "login", testSessionFlag, filepath.Join(t.TempDir(), "session.json")},
		io.NopCloser(strings.NewReader("account\npassword\ncode\n")),
		func(string) string { return "" }, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}

	if probe.selectedPhone == nil {
		t.Fatal("SDK default selected the first, voice-only phone")
	}

	selected, err := probe.selectedPhone.AsTrustedPhoneNumberID0()
	if err != nil || selected != 2 {
		t.Fatal("did not select the SMS-capable phone")
	}
}

func (p *interactiveAuthProbe) VerifyTwoFactorCode(_ context.Context, req icloud.VerifyTwoFactorCodeRequest,
) (*icloud.NativeAuthResult, error) {
	p.verification = req
	req.State.RequiresMFA = false

	var result icloud.NativeAuthResult

	result.Success, result.TrustedSession, result.State = true, true, req.State

	return &result, nil
}

func TestAuthLoginContinuesPendingLoginAndDoesNotReadAhead(t *testing.T) {
	t.Parallel()

	var (
		probe              interactiveAuthProbe
		output, diagnostic bytes.Buffer
	)

	input := io.NopCloser(strings.NewReader("synthetic@example.test\nsynthetic-password\n123456\n"))

	err := command.RunWithInput(t.Context(), &probe,
		[]string{"auth", "login", testSessionFlag, filepath.Join(t.TempDir(), "session.json")},
		input, func(string) string { return "" }, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}

	if probe.login.AccountName != "synthetic@example.test" || probe.login.Password != "synthetic-password" ||
		probe.login.PauseTwoFactor {
		t.Fatal("interactive login did not send the expected SDK request")
	}

	if probe.verification.Code != "123456" || probe.verification.Auth.ClientID != interactiveClientID {
		t.Fatal("verification lost original challenge or read-ahead consumed code")
	}

	console := output.String() + diagnostic.String()
	if strings.Contains(console, "synthetic-password") || strings.Contains(console, "123456") {
		t.Fatal("console disclosed a secret")
	}
}

func TestAuthLoginHelpNeedsNoCredentials(t *testing.T) {
	t.Parallel()

	var output, diagnostic bytes.Buffer

	err := command.Run(t.Context(), nil, []string{"auth", "login", "--help"}, &output, &diagnostic)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(diagnostic.String(), "go-icloud auth login") {
		t.Fatal("missing nested command help")
	}
}
