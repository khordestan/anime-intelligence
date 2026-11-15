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
	return &Message{Purpose: PasswordReset, Recipient: email, Token: "secret-token", ExpiresAt: time.Now().Add(time.Minute)}, nil
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
	if !s.RequestPasswordReset(context.Background(), "synthetic@example.test") {
		t.Fatal("initial enqueue rejected")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	for range 32 {
		if !s.RequestPasswordReset(context.Background(), "synthetic@example.test") {
			t.Fatal("queue ended too soon")
		}
	}
	if s.RequestPasswordReset(context.Background(), "synthetic@example.test") {
		t.Fatal("unbounded queue")
	}
	s.Close()
	if s.RequestPasswordReset(context.Background(), "synthetic@example.test") {
		t.Fatal("closed worker accepted work")
	}
}
func TestDeliveryErrorsAndEnvelopesDoNotLogSecrets(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	delivered := make(chan Message, 1)
	s := NewService(issuerStub{}, senderStub{delivered: delivered, err: errors.New("secret-token-and-email")}, logger)
	if !s.RequestPasswordReset(context.Background(), "synthetic@example.test") {
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

func TestServiceRetriesDeliveryWithoutReissuingChallenge(t *testing.T) {
	issuer := &countingIssuer{}
	delivered := make(chan Message, 3)
	attempts := 0 // Only accessed by the service's single worker.
	sender := senderFunc(func(_ context.Context, m Message) error {
		attempts++
		delivered <- m
		if attempts < 3 {
			return errors.New("private-provider-response")
		}
		return nil
	})
	service := NewService(issuer, sender, slog.Default())
	defer service.Close()
	if !service.RequestPasswordReset(context.Background(), "synthetic@example.test") {
		t.Fatal("request rejected")
	}
	var first Message
	for i := 0; i < 3; i++ {
		select {
		case m := <-delivered:
			if i == 0 {
				first = m
			} else if m != first {
				t.Fatal("delivery reissued the proof")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("retry did not run")
		}
	}
	service.Close()
	if issuer.calls != 1 {
		t.Fatalf("issued %d challenges, want one", issuer.calls)
	}
}

type countingIssuer struct{ calls int }

func (i *countingIssuer) IssuePasswordReset(_ context.Context, _ string) (*Message, error) {
	i.calls++
	message := retryMessage()
	return &message, nil
}
func (i *countingIssuer) IssueEmailVerification(ctx context.Context, token string) (*Message, error) {
	return i.IssuePasswordReset(ctx, token)
}

type durableStub struct {
	issuerStub
	submit func(context.Context, string, string) error
}

func (d durableStub) SubmitRequest(ctx context.Context, purpose, value string) error {
	return d.submit(ctx, purpose, value)
}
func (durableStub) ProcessRequest(context.Context) (bool, error)      { return false, nil }
func (durableStub) DeliverNext(context.Context, Sender) (bool, error) { return false, nil }
func TestDurableAcceptanceWaitsForPersistenceAndShutdownCancelsIntake(t *testing.T) {
	for _, mode := range []string{"commit", "shutdown", "request-cancel"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{})
			commit := make(chan struct{})
			result := make(chan bool, 1)
			backend := durableStub{submit: func(ctx context.Context, purpose, value string) error {
				close(started)
				select {
				case <-commit:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}}
			s := NewDurableService(backend, senderStub{}, slog.Default())
			defer s.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() { result <- s.RequestPasswordReset(ctx, "synthetic@example.test") }()
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("intake not started")
			}
			select {
			case <-result:
				t.Fatal("acknowledged before persistence")
			default:
			}
			switch mode {
			case "commit":
				close(commit)
			case "shutdown":
				s.Close()
			case "request-cancel":
				cancel()
			}
			select {
			case accepted := <-result:
				if accepted != (mode == "commit") {
					t.Fatal("incorrect acceptance", accepted)
				}
			case <-time.After(time.Second):
				t.Fatal("cancelled intake did not finish")
			}
			s.Close()
			if s.RequestPasswordReset(context.Background(), "synthetic@example.test") {
				t.Fatal("closed service accepted work")
			}
		})
	}
}
