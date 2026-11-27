package identitymail

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/smtptransport"
	"github.com/khordestan/anime-intelligence/internal/testutil/mailbox"
)

func TestSMTPVerifiedTLSAndPrivateFragment(t *testing.T) {
	server, err := mailbox.New("127.0.0.1:0", "127.0.0.1:0", "synthetic-user", "synthetic-password", strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err = os.WriteFile(ca, server.CAPEM, 0600); err != nil {
		t.Fatal(err)
	}
	var outcomes []smtptransport.Outcome
	config := SMTPConfig{Observe: func(outcome smtptransport.Outcome) { outcomes = append(outcomes, outcome) }, Address: server.SMTPAddress(), Username: "synthetic-user", Password: "synthetic-password", From: "sender@example.test", AppOrigin: "https://app.example.test", CAFile: ca}
	sender, err := NewSMTP(config)
	if err != nil {
		t.Fatal(err)
	}
	message := Message{Purpose: PasswordReset, Recipient: "recipient@example.test", Token: strings.Repeat("A", 43), ExpiresAt: time.Now().Add(time.Hour)}
	if err = sender.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	messages := server.Messages()
	if len(messages) != 1 || messages[0].Recipient != message.Recipient {
		t.Fatal("SMTP delivery contract")
	}
	body := messages[0].Body
	if !strings.Contains(body, "https://app.example.test/#action=reset-password&token=") || !strings.Contains(body, message.Token) || strings.Contains(body, "?token=") {
		t.Fatal("token not isolated in URL fragment")
	}
	client := http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + server.HTTPAddress() + "/messages?recipient=recipient@example.test")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("test mailbox exposed without its explicit key")
	}
	config.CAFile = ""
	untrusted, err := NewSMTP(config)
	if err != nil {
		t.Fatal(err)
	}
	if err = untrusted.Send(context.Background(), message); !errors.Is(err, ErrDelivery) {
		t.Fatal("untrusted certificate was accepted", err)
	}
	config.CAFile = ca
	config.Password = "incorrect"
	badAuth, err := NewSMTP(config)
	if err != nil {
		t.Fatal(err)
	}
	if err = badAuth.Send(context.Background(), message); !errors.Is(err, ErrDelivery) {
		t.Fatal("bad SMTP credentials accepted", err)
	}
	if len(server.Messages()) != 1 {
		t.Fatal("failed attempts delivered mail")
	}
	if !slices.Equal(outcomes, []smtptransport.Outcome{smtptransport.Accepted, smtptransport.ConnectionError, smtptransport.AuthenticationError}) {
		t.Fatalf("unexpected redacted SMTP outcomes: %v", outcomes)
	}
}
func TestSMTPRejectsUnsafeConfigurationAndEnvelopes(t *testing.T) {
	valid := SMTPConfig{Address: "localhost:465", Username: "synthetic", Password: "synthetic", From: "sender@example.test", AppOrigin: "https://app.example.test"}
	for _, mutate := range []func(*SMTPConfig){func(c *SMTPConfig) { c.AppOrigin = "http://public.example.test" }, func(c *SMTPConfig) { c.AppOrigin = "https://user:pass@app.example.test" }, func(c *SMTPConfig) { c.AppOrigin = "https://app.example.test/path" }, func(c *SMTPConfig) { c.AppOrigin = "https://app.example.test/?token=x" }, func(c *SMTPConfig) { c.From = "sender@example.test\r\nBcc:other@example.test" }, func(c *SMTPConfig) { c.Address = "localhost:not-a-port" }, func(c *SMTPConfig) { c.Password = "" }} {
		c := valid
		mutate(&c)
		if _, err := NewSMTP(c); err == nil {
			t.Fatal("unsafe config accepted")
		}
	}
	sender, err := NewSMTP(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []Message{{Purpose: "invalid", Recipient: "recipient@example.test", Token: strings.Repeat("A", 43)}, {Purpose: PasswordReset, Recipient: "recipient@example.test\r\nBcc:other@example.test", Token: strings.Repeat("A", 43)}, {Purpose: EmailVerification, Recipient: "recipient@example.test", Token: "bad-token"}} {
		if _, err = sender.render(message); err == nil {
			t.Fatal("unsafe envelope accepted")
		}
	}
	if _, err = mailbox.New("0.0.0.0:0", "127.0.0.1:0", "synthetic", "synthetic", strings.Repeat("k", 32)); err == nil {
		t.Fatal("test mailbox bound a public interface")
	}
}
