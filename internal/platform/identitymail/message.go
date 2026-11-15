// Package identitymail defines the private identity-delivery boundary. Messages
// are never API response objects and must not be written to application logs.
package identitymail

import (
	"context"
	"log/slog"
	"time"
)

const PasswordReset = "PASSWORD_RESET"
const EmailVerification = "EMAIL_VERIFICATION"

type Message struct {
	Purpose   string    `json:"-"`
	Recipient string    `json:"-"`
	Token     string    `json:"-"`
	ExpiresAt time.Time `json:"-"`
}

func (m Message) LogValue() slog.Value {
	return slog.GroupValue(slog.String("purpose", m.Purpose), slog.Time("expires_at", m.ExpiresAt))
}
func (m Message) String() string { return "identity mail (redacted)" }

type Sender interface {
	Send(context.Context, Message) error
}
