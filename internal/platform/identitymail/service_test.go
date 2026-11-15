package identitymail

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type issuerStub struct {
	started chan struct{}
	fail    bool
}

func (i issuerStub) IssuePasswordReset(ctx context.Context, email string) (*Message, error) {
	if i.started != nil {
		select {
		case i.started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if i.fail {
		return nil, errors.New("secret-token-and-email")
	}
	return &Message{Purpose: PasswordReset, Recipient: email, Token: "secret-token"}, nil
}
func (i issuerStub) IssueEmailVerification(ctx context.Context, token string) (*Message, error) {
	return i.IssuePasswordReset(ctx, token)
}

type senderStub struct {
	delivered chan Message
	err       error
}

func (s senderStub) Send(ctx context.Context, m Message) error {
	if s.delivered != nil {
		s.delivered <- m
	}
	return s.err
}
func TestDeliveryQueueIsBoundedAndCancellationStopsWork(t *testing.T) {
	started := make(chan struct{}, 1)
	s := NewService(issuerStub{started: started}, senderStub{}, slog.Default())
	t.Cleanup(s.Close)
	if !s.RequestPasswordReset("synthetic@example.test") {
		t.Fatal("initial enqueue rejected")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	for range 32 {
		if !s.RequestPasswordReset("synthetic@example.test") {
			t.Fatal("queue ended too soon")
		}
	}
	if s.RequestPasswordReset("synthetic@example.test") {
		t.Fatal("unbounded queue")
	}
	s.Close()
	if s.RequestPasswordReset("synthetic@example.test") {
		t.Fatal("closed worker accepted work")
	}
}
func TestDeliveryErrorsAndEnvelopesDoNotLogSecrets(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	delivered := make(chan Message, 1)
	s := NewService(issuerStub{}, senderStub{delivered: delivered, err: errors.New("secret-token-and-email")}, logger)
	if !s.RequestPasswordReset("synthetic@example.test") {
		t.Fatal("enqueue")
	}
	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("delivery did not run")
	}
	// Close waits for the worker before reading the buffer, avoiding a log race.
	s.Close()
	logger.Info("test envelope", "message", Message{Purpose: PasswordReset, Recipient: "synthetic@example.test", Token: "secret-token"})
	if strings.Contains(logs.String(), "secret-token") || strings.Contains(logs.String(), "synthetic@example.test") {
		t.Fatal("delivery logging disclosed private fields")
	}
}
