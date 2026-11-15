package main

import (
	"strings"
	"testing"
)

func TestMailConfigurationCannotFallBackToJSON(t *testing.T) {
	settings := map[string]string{"MAIL_SMTP_ADDR": "smtp.example.test:465", "MAIL_SMTP_USER": "synthetic-user", "MAIL_SMTP_PASSWORD": "synthetic-secret", "MAIL_FROM": "sender@example.test", "PUBLIC_APP_ORIGIN": "https://app.example.test", "MAIL_OUTBOX_KEY": strings.Repeat("A", 43) + "="}
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

func TestMailRequiresExplicitOutboxKey(t *testing.T) {
	settings := map[string]string{"MAIL_SMTP_ADDR": "smtp.example.test:465", "MAIL_SMTP_USER": "synthetic", "MAIL_SMTP_PASSWORD": "synthetic", "MAIL_FROM": "sender@example.test", "PUBLIC_APP_ORIGIN": "https://app.example.test"}
	for _, key := range []string{"", "not-a-key", strings.Repeat("A", 32)} {
		settings["MAIL_OUTBOX_KEY"] = key
		if sender, err := identitySenderFromEnv(func(k string) string { return settings[k] }, true); err == nil || sender != nil {
			t.Fatal("mail accepted without valid outbox key")
		}
	}
}

func TestMailKeyringValidationAndPartialConfiguration(t *testing.T) {
	primary := strings.Repeat("A", 43) + "="
	previous := "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="
	settings := map[string]string{"MAIL_SMTP_ADDR": "smtp.example.test:465", "MAIL_SMTP_USER": "synthetic", "MAIL_SMTP_PASSWORD": "synthetic", "MAIL_FROM": "sender@example.test", "PUBLIC_APP_ORIGIN": "https://app.example.test", "MAIL_OUTBOX_KEY": primary, "MAIL_OUTBOX_PREVIOUS_KEYS": previous}
	getenv := func(key string) string { return settings[key] }
	if sender, err := identitySenderFromEnv(getenv, true); err != nil || sender == nil {
		t.Fatal("valid keyring rejected", err)
	}
	for _, keys := range []string{primary, previous + "," + previous, previous + ",", strings.Join([]string{previous, previous, previous, previous}, ",")} {
		settings["MAIL_OUTBOX_PREVIOUS_KEYS"] = keys
		if sender, err := identitySenderFromEnv(getenv, true); err == nil || sender != nil || strings.Contains(err.Error(), previous) {
			t.Fatal("invalid keyring accepted or disclosed")
		}
	}
	if sender, err := identitySenderFromEnv(func(key string) string {
		if key == "MAIL_OUTBOX_PREVIOUS_KEYS" {
			return previous
		}
		return ""
	}, true); err == nil || sender != nil {
		t.Fatal("previous keys silently ignored without primary/mail configuration")
	}
}
