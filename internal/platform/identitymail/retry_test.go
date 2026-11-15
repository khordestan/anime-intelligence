package identitymail

import (
	"context"
	"errors"
	"testing"
	"time"
)

type senderFunc func(context.Context, Message) error

func (f senderFunc) Send(ctx context.Context, m Message) error { return f(ctx, m) }

func retryMessage() Message {
	return Message{Purpose: PasswordReset, Recipient: "synthetic@example.test", Token: "synthetic-proof", ExpiresAt: time.Now().Add(time.Minute)}
}
func TestRetryReusesProofAndStopsAfterAcceptance(t *testing.T) {
	for _, failures := range []int{0, 1, 2, 3} {
		t.Run(string(rune('0'+failures)), func(t *testing.T) {
			message := retryMessage()
			attempts := 0
			sender := senderFunc(func(ctx context.Context, got Message) error {
				attempts++
				if got != message {
					t.Fatal("retry changed issued proof")
				}
				deadline, ok := ctx.Deadline()
				if !ok || deadline.After(message.ExpiresAt) {
					t.Fatal("attempt outlives proof")
				}
				if attempts <= failures {
					return errors.New("private provider response")
				}
				return nil
			})
			err := sendWithRetry(context.Background(), sender, message, time.Millisecond)
			want := failures + 1
			if want > 3 {
				want = 3
			}
			if attempts != want {
				t.Fatalf("attempts = %d, want %d", attempts, want)
			}
			if failures == 3 && err != ErrDelivery {
				t.Fatal("exhaustion did not redact provider failure")
			}
			if failures < 3 && err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestRetryStopsAtExpiryOrCancellation(t *testing.T) {
	for _, mode := range []string{"already-expired", "already-cancelled", "backoff-cancelled", "backoff-expired", "in-flight-cancelled"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			message := retryMessage()
			if mode == "already-expired" {
				message.ExpiresAt = time.Now().Add(-time.Second)
			}
			if mode == "already-cancelled" {
				cancel()
			}
			if mode == "backoff-expired" {
				message.ExpiresAt = time.Now().Add(20 * time.Millisecond)
			}
			attempts := 0
			sender := senderFunc(func(ctx context.Context, _ Message) error {
				attempts++
				if mode == "backoff-cancelled" {
					cancel()
				}
				if mode == "in-flight-cancelled" {
					cancel()
					<-ctx.Done()
				}
				return errors.New("private failure")
			})
			start := time.Now()
			if err := sendWithRetry(ctx, sender, message, time.Hour); err != ErrDelivery {
				t.Fatal(err)
			}
			want := 1
			if mode == "already-expired" || mode == "already-cancelled" {
				want = 0
			}
			if attempts != want {
				t.Fatalf("attempts=%d want=%d", attempts, want)
			}
			if time.Since(start) > time.Second {
				t.Fatal("shutdown or expiry waited for backoff")
			}
		})
	}
}
