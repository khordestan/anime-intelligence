// Package smtptransport delivers application-rendered mail over authenticated,
// certificate-verified implicit TLS. It has no access to account storage.
package smtptransport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

var ErrConfig = errors.New("invalid SMTP configuration")
var ErrDelivery = errors.New("SMTP delivery failed")

type Outcome uint8

const (
	Accepted Outcome = iota
	ConnectionError
	GreetingError
	AuthenticationError
	EnvelopeError
	DataError
)

type Config struct {
	Address  string
	Username string
	Password string
	From     string
	CAFile   string
	Observe  func(Outcome)
}

type Client struct {
	config Config
	host   string
	roots  *x509.CertPool
}

func ValidMailbox(address string) bool {
	parsed, err := mail.ParseAddress(address)
	return err == nil && parsed.Address == address && !strings.ContainsAny(address, "\r\n")
}

func New(config Config) (*Client, error) {
	host, port, err := net.SplitHostPort(config.Address)
	if err != nil || host == "" || port == "" || config.Username == "" || config.Password == "" ||
		!ValidMailbox(config.From) {
		return nil, ErrConfig
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
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
	return &Client{config: config, host: host, roots: roots}, nil
}

// Send reduces all provider errors to a fixed sentinel for callers.
func (s *Client) Send(ctx context.Context, recipient string, data []byte) error {
	if !ValidMailbox(recipient) {
		return ErrDelivery
	}
	outcome := ConnectionError
	if s.config.Observe != nil {
		defer func() { s.config.Observe(outcome) }()
	}
	dialer := tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 5 * time.Second},
		Config:    &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.host, RootCAs: s.roots},
	}
	conn, err := dialer.DialContext(ctx, "tcp", s.config.Address)
	if err != nil {
		return ErrDelivery
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline := time.Now().Add(10 * time.Second)
	if incoming, ok := ctx.Deadline(); ok && incoming.Before(deadline) {
		deadline = incoming
	}
	_ = conn.SetDeadline(deadline)
	outcome = GreetingError
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return ErrDelivery
	}
	defer client.Close()
	outcome = AuthenticationError
	if err = client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.host)); err != nil {
		return ErrDelivery
	}
	outcome = EnvelopeError
	if err = client.Mail(s.config.From); err != nil {
		return ErrDelivery
	}
	if err = client.Rcpt(recipient); err != nil {
		return ErrDelivery
	}
	outcome = DataError
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
	// DATA acceptance marks delivery even if the server later rejects QUIT.
	outcome = Accepted
	_ = client.Quit()
	return nil
}
