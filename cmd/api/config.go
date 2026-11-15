package main

import (
	"github.com/khordestan/anime-intelligence/internal/platform/identitymail"
	"strings"
)

// Mail configuration must not be ignored by falling back to unauthenticated
// JSON mode. Constructing the adapter validates config without sending mail.
func identitySenderFromEnv(getenv func(string) string, databaseEnabled bool) (*identitymail.SMTP, error) {
	config := identitymail.SMTPConfig{Address: getenv("MAIL_SMTP_ADDR"), Username: getenv("MAIL_SMTP_USER"), Password: getenv("MAIL_SMTP_PASSWORD"), From: getenv("MAIL_FROM"), AppOrigin: getenv("PUBLIC_APP_ORIGIN"), CAFile: getenv("MAIL_SMTP_CA_FILE")}
	configured := config.Address != "" || config.Username != "" || config.Password != "" || config.From != "" || config.AppOrigin != "" || config.CAFile != "" || getenv("MAIL_OUTBOX_KEY") != "" || getenv("MAIL_OUTBOX_PREVIOUS_KEYS") != ""
	if !configured {
		return nil, nil
	}
	if !databaseEnabled {
		return nil, identitymail.ErrConfig
	}
	if _, err := identityCipherFromEnv(getenv); err != nil {
		return nil, identitymail.ErrConfig
	}
	return identitymail.NewSMTP(config)
}

func identityCipherFromEnv(getenv func(string) string) (*identitymail.EnvelopeCipher, error) {
	var previous []string
	if encoded := getenv("MAIL_OUTBOX_PREVIOUS_KEYS"); encoded != "" {
		previous = strings.Split(encoded, ",")
	}
	return identitymail.NewEnvelopeKeyring(getenv("MAIL_OUTBOX_KEY"), previous)
}
