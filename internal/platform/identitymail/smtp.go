package identitymail

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrDelivery = errors.New("identity mail delivery failed")
var ErrConfig = errors.New("invalid identity mail configuration")

type SMTPConfig struct{ Address, Username, Password, From, AppOrigin, CAFile string }
type SMTP struct {
	config SMTPConfig
	host   string
	origin *url.URL
	roots  *x509.CertPool
}

func NewSMTP(config SMTPConfig) (*SMTP, error) {
	host, port, err := net.SplitHostPort(config.Address)
	if err != nil || host == "" || port == "" || config.Username == "" || config.Password == "" {
		return nil, ErrConfig
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, ErrConfig
	}
	from, err := mail.ParseAddress(config.From)
	if err != nil || from.Address != config.From || strings.ContainsAny(config.From, "\r\n") {
		return nil, ErrConfig
	}
	origin, err := url.Parse(config.AppOrigin)
	if err != nil || origin.Hostname() == "" || origin.User != nil || origin.RawPath != "" || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") || origin.Opaque != "" {
		return nil, ErrConfig
	}
	ip := net.ParseIP(origin.Hostname())
	loopback := origin.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if origin.Scheme != "https" && !(origin.Scheme == "http" && loopback) {
		return nil, ErrConfig
	}
	var roots *x509.CertPool
	if config.CAFile != "" {
		roots, err = x509.SystemCertPool()
		if err != nil {
			return nil, ErrConfig
		}
		pem, err := os.ReadFile(config.CAFile)
		if err != nil || !roots.AppendCertsFromPEM(pem) {
			return nil, ErrConfig
		}
	}
	return &SMTP{config: config, host: host, origin: origin, roots: roots}, nil
}
func (s *SMTP) render(message Message) ([]byte, error) {
	to, err := mail.ParseAddress(message.Recipient)
	if err != nil || to.Address != message.Recipient || strings.ContainsAny(message.Recipient, "\r\n") {
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
	if _, err = rand.Read(id); err != nil {
		return nil, ErrDelivery
	}
	domain := s.config.From[strings.LastIndex(s.config.From, "@")+1:]
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@%s>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n", s.config.From, message.Recipient, subject, time.Now().UTC().Format(time.RFC1123Z), hex.EncodeToString(id), domain)
	return []byte(headers + body), nil
}

// Send always uses authenticated, certificate-verified implicit TLS. Provider
// errors are deliberately reduced to a fixed sentinel, never logged verbatim.
func (s *SMTP) Send(ctx context.Context, message Message) error {
	data, err := s.render(message)
	if err != nil {
		return ErrDelivery
	}
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 5 * time.Second}, Config: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.host, RootCAs: s.roots}}
	conn, err := dialer.DialContext(ctx, "tcp", s.config.Address)
	if err != nil {
		return ErrDelivery
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return ErrDelivery
	}
	defer client.Close()
	if err = client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.host)); err != nil {
		return ErrDelivery
	}
	if err = client.Mail(s.config.From); err != nil {
		return ErrDelivery
	}
	if err = client.Rcpt(message.Recipient); err != nil {
		return ErrDelivery
	}
	writer, err := client.Data()
	if err != nil {
		return ErrDelivery
	}
	if _, err = writer.Write(data); err != nil {
		_ = writer.Close()
		return ErrDelivery
	}
	if err = writer.Close(); err != nil {
		return ErrDelivery
	}
	// DATA acceptance is the delivery boundary; a failed QUIT must not trigger a
	// duplicate send after the server has accepted the message.
	_ = client.Quit()
	return nil
}
