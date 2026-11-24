package identitymail

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/smtptransport"
)

var ErrDelivery = errors.New("identity mail delivery failed")
var ErrConfig = errors.New("invalid identity mail configuration")

type SMTPConfig struct{ Address, Username, Password, From, AppOrigin, CAFile string }
type SMTP struct {
	from      string
	origin    *url.URL
	transport *smtptransport.Client
}

func NewSMTP(config SMTPConfig) (*SMTP, error) {
	origin, err := url.Parse(config.AppOrigin)
	if err != nil || origin.Hostname() == "" || origin.User != nil || origin.RawPath != "" || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") || origin.Opaque != "" {
		return nil, ErrConfig
	}
	ip := net.ParseIP(origin.Hostname())
	loopback := origin.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if origin.Scheme != "https" && !(origin.Scheme == "http" && loopback) {
		return nil, ErrConfig
	}
	transport, err := smtptransport.New(smtptransport.Config{
		Address: config.Address, Username: config.Username, Password: config.Password,
		From: config.From, CAFile: config.CAFile,
	})
	if err != nil {
		return nil, ErrConfig
	}
	return &SMTP{from: config.From, origin: origin, transport: transport}, nil
}

func (s *SMTP) render(message Message) ([]byte, error) {
	if !smtptransport.ValidMailbox(message.Recipient) {
		return nil, ErrDelivery
	}
	action, subject := "reset-password", "Reset your Anime Intelligence password"
	switch message.Purpose {
	case PasswordReset:
	case EmailVerification:
		action, subject = "verify-email", "Verify your Anime Intelligence email"
	default:
		return nil, ErrDelivery
	}
	decoded, tokenErr := base64.RawURLEncoding.Strict().DecodeString(message.Token)
	if tokenErr != nil || len(decoded) != 32 || len(message.Token) != 43 {
		return nil, ErrDelivery
	}
	link := *s.origin
	link.Path = "/"
	link.Fragment = url.Values{"action": {action}, "token": {message.Token}}.Encode()
	// Fragment credentials are not sent in HTTP URLs/referrers. The browser must
	// scrub the fragment and require an explicit confirmation before redemption.
	body := fmt.Sprintf("Use this link to continue:\r\n%s\r\n\r\nThis link expires at %s. If you did not request it, you can ignore this message.\r\n", link.String(), message.ExpiresAt.UTC().Format(time.RFC3339))
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, ErrDelivery
	}
	domain := s.from[strings.LastIndex(s.from, "@")+1:]
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@%s>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n", s.from, message.Recipient, subject, time.Now().UTC().Format(time.RFC1123Z), hex.EncodeToString(id), domain)
	return []byte(headers + body), nil
}

// Send always uses authenticated, certificate-verified implicit TLS. Provider
// errors are deliberately reduced to a fixed sentinel, never logged verbatim.
func (s *SMTP) Send(ctx context.Context, message Message) error {
	data, err := s.render(message)
	if err != nil {
		return ErrDelivery
	}
	if err := s.transport.Send(ctx, message.Recipient, data); err != nil {
		return ErrDelivery
	}
	return nil
}
