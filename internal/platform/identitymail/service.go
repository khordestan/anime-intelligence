package identitymail

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Issuer interface {
	IssuePasswordReset(context.Context, string) (*Message, error)
	IssueEmailVerification(context.Context, string) (*Message, error)
}
type job struct{ purpose, value string }
type Service struct {
	issuer  Issuer
	durable DurableIssuer
	sender  Sender
	logger  *slog.Logger
	jobs    chan job
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	closed  bool
}

func NewService(issuer Issuer, sender Sender, logger *slog.Logger) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{issuer: issuer, sender: sender, logger: logger, jobs: make(chan job, 32), ctx: ctx, cancel: cancel}
	s.wg.Add(1)
	go s.run()
	return s
}
func (s *Service) enqueue(ctx context.Context, purpose, value string) bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	if s.durable != nil {
		backend := s.durable
		s.wg.Add(1)
		s.mu.Unlock()
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		stop := context.AfterFunc(s.ctx, cancel)
		defer stop()
		defer cancel()
		return backend.SubmitRequest(ctx, purpose, value) == nil
	}
	defer s.mu.Unlock()
	select {
	case s.jobs <- job{purpose, value}:
		return true
	default:
		return false
	}
}

// Enqueue before account lookup: acceptance does not depend on account state or
// provider latency. The configured durable service commits an encrypted intent;
// NewService retains a volatile adapter for local tests.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) bool {
	if len(email) > 254 {
		return false
	}
	return s.enqueue(ctx, PasswordReset, email)
}
func (s *Service) RequestVerification(ctx context.Context, token string) bool {
	if len(token) != 43 {
		return false
	}
	return s.enqueue(ctx, EmailVerification, token)
}
func (s *Service) run() {
	defer s.wg.Done()
	for {
		if s.ctx.Err() != nil {
			return
		}
		select {
		case <-s.ctx.Done():
			return
		case j := <-s.jobs:
			func() {
				ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
				var message *Message
				var err error
				if j.purpose == PasswordReset {
					message, err = s.issuer.IssuePasswordReset(ctx, j.value)
				} else {
					message, err = s.issuer.IssueEmailVerification(ctx, j.value)
				}
				cancel()
				if err == nil && message != nil {
					err = sendWithRetry(s.ctx, s.sender, *message, time.Second)
				}
				if err != nil && s.ctx.Err() == nil {
					s.logger.Warn("identity delivery attempt failed")
				}
			}()
		}
	}
}
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
	for {
		select {
		case <-s.jobs:
		default:
			return
		}
	}
}
