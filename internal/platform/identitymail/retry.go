package identitymail

import (
	"context"
	"time"
)

// Retry the issued proof, never issuance: issuing another proof would revoke the
// first and consume the recipient's resend quota. This remains volatile work.
func sendWithRetry(ctx context.Context, sender Sender, message Message, baseDelay time.Duration) error {
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil || !time.Now().Before(message.ExpiresAt) {
			return ErrDelivery
		}
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1))
			if remaining := time.Until(message.ExpiresAt); remaining < delay {
				delay = remaining
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ErrDelivery
			case <-timer.C:
			}
			if ctx.Err() != nil || !time.Now().Before(message.ExpiresAt) {
				return ErrDelivery
			}
		}
		deadline := time.Now().Add(15 * time.Second)
		if message.ExpiresAt.Before(deadline) {
			deadline = message.ExpiresAt
		}
		attemptCtx, cancel := context.WithDeadline(ctx, deadline)
		err := sender.Send(attemptCtx, message)
		cancel()
		if err == nil {
			return nil
		}
	}
	return ErrDelivery
}
