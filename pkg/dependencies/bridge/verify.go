package bridge

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/portpowered/go-icloud/pkg/dependencies/bridgeprover"
	models "github.com/portpowered/go-icloud/pkg/dependencymodels/bridge"
)

var errVerification = errors.New("bridge challenge is not ready for code verification")

// Snapshot returns an independent copy of the latest complete challenge envelope.
func (session *Session) Snapshot() (*Push, error) {
	session.mutex.Lock()
	push := session.push
	session.mutex.Unlock()

	if push == nil {
		return nil, ErrClosed
	}

	encoded, err := json.Marshal(push.Payload)
	if err != nil {
		return nil, fmt.Errorf("copy bridge push: %w", err)
	}

	var copied models.BridgePushPayload

	err = json.Unmarshal(encoded, &copied)
	if err != nil {
		return nil, fmt.Errorf("copy bridge push: %w", err)
	}

	return &Push{Payload: copied, SessionID: push.SessionID, NextStep: push.NextStep}, nil
}

// VerifyCode completes the native SPAKE2 exchange and always closes its socket.
func (session *Session) VerifyCode(ctx context.Context, code string) (bool, error) {
	if !session.busy.CompareAndSwap(false, true) {
		return false, ErrBusy
	}
	defer session.busy.Store(false)
	defer func() { _ = session.Close() }()

	challenge, err := session.activeChallenge()
	if err != nil {
		return false, err
	}

	push := challenge.push

	legacy := strings.HasSuffix(valueOrEmpty(push.Payload.Txnid), string(models.LegacyTransactionSuffix))

	if push.NextStep != fmt.Sprint(models.ProverShareStep) || push.Payload.Salt == nil || legacy {
		return false, &ProtocolError{Stage: stageVerification, Cause: errVerification}
	}

	work, cancel := context.WithCancel(ctx)

	//nolint:contextcheck // The stored session owner cancels work when its connection closes.
	stop := context.AfterFunc(session.ctx, cancel)

	closeOnCancel := context.AfterFunc(work, func() { _ = session.Close() })
	defer closeOnCancel()
	defer stop()
	defer cancel()

	err = work.Err()
	if err != nil {
		return false, &ProtocolError{Stage: stageVerification, Cause: err}
	}

	return session.verify(work, challenge.socket, push, code)
}

type challengeState struct {
	socket Socket
	push   *Push
}

func (session *Session) activeChallenge() (*challengeState, error) {
	session.mutex.Lock()
	socket := session.socket
	session.mutex.Unlock()

	if socket == nil {
		return nil, ErrClosed
	}

	push, err := session.Snapshot()
	if err != nil {
		return nil, err
	}

	return &challengeState{socket: socket, push: push}, nil
}

func (session *Session) verify(ctx context.Context, socket Socket, push *Push, code string) (bool, error) {
	prover, err := session.beginProof(ctx, push, code)
	if err != nil {
		return false, err
	}

	push, err = session.next(ctx, socket)
	if err != nil {
		return false, err
	}

	if push.NextStep != fmt.Sprint(models.ProverConfirmationStep) || push.Payload.Data == nil {
		return false, &ProtocolError{Stage: "step 4", Cause: errVerification}
	}

	confirmation, verified, err := processProof(prover, *push.Payload.Data)
	if err != nil || !verified {
		return false, err
	}

	err = session.exchange(ctx, models.ProverConfirmationStep, confirmation, push)
	if err != nil {
		return false, err
	}

	return session.finish(ctx, socket, prover)
}

func processProof(prover *bridgeprover.Prover, encoded string) (string, bool, error) {
	value, err := strictBase64(encoded)
	if err != nil {
		return "", false, &ProtocolError{Stage: stageServerProof, Cause: err}
	}

	parts := strings.SplitN(string(value), string(models.ProofSeparator), int(models.ProofPartCount))
	if len(parts) != int(models.ProofPartCount) {
		return "", false, &ProtocolError{Stage: stageServerProof, Cause: errPushPayload}
	}

	share, err := strictBase64(parts[0])
	if err != nil {
		return "", false, &ProtocolError{Stage: stageServerShare, Cause: err}
	}

	serverConfirmation, err := strictBase64(parts[1])
	if err != nil {
		return "", false, &ProtocolError{Stage: stageServerConfirmation, Cause: err}
	}

	confirmation, err := prover.ProcessMessage1(hex.EncodeToString(share))
	if err != nil {
		return "", false, &ProtocolError{Stage: stageServerShare, Cause: err}
	}

	_, err = prover.ProcessMessage2(hex.EncodeToString(serverConfirmation))
	if err != nil {
		if errors.Is(err, bridgeprover.ErrPayload) {
			return "", false, nil
		}

		return "", false, &ProtocolError{Stage: stageServerConfirmation, Cause: err}
	}

	result, err := hexBase64(confirmation)

	return result, err == nil, err
}

func (session *Session) finish(ctx context.Context, socket Socket, prover *bridgeprover.Prover) (bool, error) {
	push, err := session.next(ctx, socket)
	if err != nil {
		return false, err
	}

	completion := models.ProverConfirmationStep
	if push.NextStep == fmt.Sprint(models.CompletionStep) {
		completion = models.CompletionStep
	}

	if push.NextStep != fmt.Sprint(completion) || push.Payload.EncryptedCode == nil {
		return false, &ProtocolError{Stage: "final push", Cause: errVerification}
	}

	code, err := prover.DecryptMessage(*push.Payload.EncryptedCode)
	if err != nil {
		return false, &ProtocolError{Stage: "decrypt", Cause: err}
	}

	verified, err := session.options.Validate(ctx, push.SessionID, code)
	if err != nil {
		return false, err
	}

	err = session.exchange(ctx, completion, string(models.DoneDataBase64), push)
	if err != nil {
		return false, err
	}

	return verified, nil
}

func (session *Session) next(ctx context.Context, socket Socket) (*Push, error) {
	push, err := WaitPush(ctx, socket, session.topic, session.waitOptions())
	if err != nil {
		return nil, err
	}

	rejected := push.Payload.Ec != nil && *push.Payload.Ec != int(models.BridgePushErrorSuccess)
	if push.SessionID != session.identifier || rejected {
		return nil, &ProtocolError{Stage: "challenge", Cause: errSession}
	}

	session.mutex.Lock()
	session.push = push
	session.mutex.Unlock()

	return push, nil
}

func (session *Session) exchange(ctx context.Context, step models.BridgeStep, data string, push *Push) error {
	err := ctx.Err()
	if err != nil {
		return &ProtocolError{Stage: stageHTTPExchange, Cause: err}
	}

	request := new(models.BridgeExchange)
	request.NextStep, request.SessionUUID, request.Ptkn = step, push.SessionID, session.token
	request.Data, request.Idmsdata, request.Akdata = &data, push.Payload.Idmsdata, push.Payload.Akdata

	err = session.options.Exchange(ctx, *request)
	if err != nil {
		return &ProtocolError{Stage: stageHTTPExchange, Cause: err}
	}

	return nil
}

func hexBase64(value string) (string, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return "", &ProtocolError{Stage: "proof encoding", Cause: err}
	}

	return base64.StdEncoding.EncodeToString(decoded), nil
}

func (session *Session) beginProof(ctx context.Context, push *Push, code string) (*bridgeprover.Prover, error) {
	prover := bridgeprover.New(session.options.Entropy)

	err := prover.InitContext(ctx, *push.Payload.Salt, code)
	if err != nil {
		return nil, &ProtocolError{Stage: "prover", Cause: err}
	}

	share, err := prover.Message1()
	if err != nil {
		return nil, &ProtocolError{Stage: "prover share", Cause: err}
	}

	encoded, err := hexBase64(share)
	if err != nil {
		return nil, err
	}

	err = session.exchange(ctx, models.ProverShareStep, encoded, push)
	if err != nil {
		return nil, err
	}

	return prover, nil
}
