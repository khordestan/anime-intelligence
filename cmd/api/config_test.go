package main

import (
	"strings"
	"testing"
)

func TestMailConfigurationCannotFallBackToJSON(t *testing.T) {
	settings := map[string]string{"MAIL_SMTP_ADDR": "smtp.example.test:465", "MAIL_SMTP_USER": "synthetic-user", "MAIL_SMTP_PASSWORD": "synthetic-secret", "MAIL_FROM": "sender@example.test", "PUBLIC_APP_ORIGIN": "https://app.example.test"}
	getenv := func(key string) string { return settings[key] }
	if sender, err := identitySenderFromEnv(getenv, false); err == nil || sender != nil {
		t.Fatal("mail was accepted without a PostgreSQL runtime")
	}
	if sender, err := identitySenderFromEnv(getenv, true); err != nil || sender == nil {
		t.Fatal("valid explicit config rejected", err)
	}
	delete(settings, "MAIL_FROM")
	if sender, err := identitySenderFromEnv(getenv, true); err == nil || sender != nil || strings.Contains(err.Error(), settings["MAIL_SMTP_PASSWORD"]) {
		t.Fatal("partial config was accepted or leaked values")
	}
	if sender, err := identitySenderFromEnv(func(string) string { return "" }, false); err != nil || sender != nil {
		t.Fatal("local JSON development mode requires no mail config", err)
	}
}
