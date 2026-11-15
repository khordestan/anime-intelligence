package identitymail

import (
	"context"
	"log/slog"
	"time"
)

// DurableIssuer persists intents before acceptance and atomically consumes them
// with proof issuance and encrypted mail. No volatile fallback is used.
type DurableIssuer interface {
	Issuer
	SubmitRequest(context.Context, string, string) error
	ProcessRequest(context.Context) (bool, error)
	DeliverNext(context.Context, Sender) (bool, error)
}

func NewDurableService(issuer DurableIssuer, sender Sender, logger *slog.Logger) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{issuer: issuer, durable: issuer, sender: sender, logger: logger, ctx: ctx, cancel: cancel}
	s.startPoller("identity request processing failed", issuer.ProcessRequest)
	s.startPoller("identity outbox delivery failed", func(ctx context.Context) (bool, error) { return issuer.DeliverNext(ctx, sender) })
	return s
}
func (s *Service) startPoller(event string, work func(context.Context) (bool, error)) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			if s.ctx.Err() != nil {
				return
			}
			ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
			worked, err := work(ctx)
			cancel()
			if err != nil && s.ctx.Err() == nil {
				s.logger.Warn(event)
			}
			if worked && err == nil {
				continue
			}
			timer := time.NewTimer(time.Second)
			select {
			case <-s.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
}
