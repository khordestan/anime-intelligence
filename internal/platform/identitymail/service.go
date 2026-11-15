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
	issuer Issuer
	sender Sender
	logger *slog.Logger
	jobs   chan job
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

func NewService(issuer Issuer, sender Sender, logger *slog.Logger) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{issuer: issuer, sender: sender, logger: logger, jobs: make(chan job, 32), ctx: ctx, cancel: cancel}
	s.wg.Add(1)
	go s.run()
	return s
}
func (s *Service) enqueue(purpose, value string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.jobs <- job{purpose, value}:
		return true
	default:
		return false
	}
}

// Enqueue before account lookup: acceptance does not depend on account state or
// provider latency. The bounded queue is volatile; durable delivery is pending.
func (s *Service) RequestPasswordReset(email string) bool {
	if len(email) > 254 {
		return false
	}
	return s.enqueue(PasswordReset, email)
}
func (s *Service) RequestVerification(token string) bool {
	if len(token) != 43 {
		return false
	}
	return s.enqueue(EmailVerification, token)
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
