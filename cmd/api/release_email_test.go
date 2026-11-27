package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khordestan/anime-intelligence/internal/testutil/mailbox"
)

func TestReleaseEmailRequiresSeparateOptInAndVerifiedTLS(t *testing.T) {
	server, err := mailbox.New("127.0.0.1:0", "127.0.0.1:0", "synthetic", "synthetic", strings.Repeat("x", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, server.CAPEM, 0600); err != nil {
		t.Fatal(err)
	}
	config := map[string]string{
		"RELEASE_EMAIL_SMTP_ADDR": server.SMTPAddress(), "RELEASE_EMAIL_SMTP_USERNAME": "synthetic",
		"RELEASE_EMAIL_SMTP_PASSWORD": "synthetic", "RELEASE_EMAIL_SMTP_FROM": "sender@example.test",
	}
	get := func(key string) string { return config[key] }
	if sender, err := releaseEmailSenderFromEnv(get, true, nil); err != nil || sender != nil {
		t.Fatalf("SMTP configuration enabled delivery by itself: %v", err)
	}
	config["RELEASE_EMAIL_ENABLED"] = "1"
	if _, err := releaseEmailSenderFromEnv(get, false, nil); err == nil {
		t.Fatal("JSON mode accepted release email")
	}
	const recipient = "recipient@example.test"
	untrusted, err := releaseEmailSenderFromEnv(get, true, nil)
	if err != nil || untrusted.send(context.Background(), recipient) == nil {
		t.Fatal("untrusted SMTP peer was not rejected")
	}
	config["RELEASE_EMAIL_SMTP_CA_FILE"] = ca
	sender, err := releaseEmailSenderFromEnv(get, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.send(context.Background(), recipient); err != nil {
		t.Fatal(err)
	}
	if messages := server.Messages(); len(messages) != 1 || messages[0].Recipient != recipient ||
		!strings.Contains(messages[0].Body, "A title you follow has a release update.") {
		t.Fatal("unexpected release message")
	}
}
