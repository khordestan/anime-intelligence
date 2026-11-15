package main

import "github.com/khordestan/anime-intelligence/internal/platform/identitymail"

// Mail configuration must not be ignored by falling back to unauthenticated
// JSON mode. Constructing the adapter validates config without sending mail.
func identitySenderFromEnv(getenv func(string) string, databaseEnabled bool) (*identitymail.SMTP, error) {
	config := identitymail.SMTPConfig{Address: getenv("MAIL_SMTP_ADDR"), Username: getenv("MAIL_SMTP_USER"), Password: getenv("MAIL_SMTP_PASSWORD"), From: getenv("MAIL_FROM"), AppOrigin: getenv("PUBLIC_APP_ORIGIN"), CAFile: getenv("MAIL_SMTP_CA_FILE")}
	configured := config.Address != "" || config.Username != "" || config.Password != "" || config.From != "" || config.AppOrigin != "" || config.CAFile != "" || getenv("MAIL_OUTBOX_KEY") != ""
	if !configured {
		return nil, nil
	}
	if !databaseEnabled {
		return nil, identitymail.ErrConfig
	}
	if _, err := identitymail.NewEnvelopeCipher(getenv("MAIL_OUTBOX_KEY")); err != nil {
		return nil, identitymail.ErrConfig
	}
	return identitymail.NewSMTP(config)
}
