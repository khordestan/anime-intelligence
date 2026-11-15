package identitymail

import (
	"context"
	"log/slog"
	"time"
)

// DurableIssuer atomically persists encrypted mail with its issued proof.
// HTTP request acceptance still uses the bounded pre-issuance queue.
type DurableIssuer interface {
	Issuer
	DeliverNext(context.Context, Sender) (bool, error)
}

func NewDurableService(issuer DurableIssuer, sender Sender, logger *slog.Logger) *Service {
	s := NewService(issuer, sender, logger)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			if s.ctx.Err() != nil {
				return
			}
			ctx, cancel := context.WithTimeout(s.ctx, 20*time.Second)
			worked, err := issuer.DeliverNext(ctx, sender)
			cancel()
			if err != nil && s.ctx.Err() == nil {
				logger.Warn("identity outbox delivery failed")
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
	return s
}
