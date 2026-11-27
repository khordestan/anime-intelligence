package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/smtptransport"
)

type releaseEmailSender struct {
	transport *smtptransport.Client
	from      string
}

func releaseEmailSenderFromEnv(getenv func(string) string, postgresql bool, observe func(smtptransport.Outcome)) (*releaseEmailSender, error) {
	switch getenv("RELEASE_EMAIL_ENABLED") {
	case "", "0":
		return nil, nil
	case "1":
		if postgresql {
			config := smtptransport.Config{
				Address: getenv("RELEASE_EMAIL_SMTP_ADDR"), Username: getenv("RELEASE_EMAIL_SMTP_USERNAME"),
				Password: getenv("RELEASE_EMAIL_SMTP_PASSWORD"), From: getenv("RELEASE_EMAIL_SMTP_FROM"),
				CAFile: getenv("RELEASE_EMAIL_SMTP_CA_FILE"), Observe: observe,
			}
			transport, err := smtptransport.New(config)
			if err != nil {
				return nil, err
			}
			return &releaseEmailSender{transport: transport, from: config.From}, nil
		}
	}
	return nil, errors.New("release email requires explicit settings and PostgreSQL")
}

func (s *releaseEmailSender) send(ctx context.Context, recipient string) error {
	if !smtptransport.ValidMailbox(recipient) {
		return smtptransport.ErrDelivery
	}
	// Keep vendor metadata and users' library titles out of SMTP content.
	message := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Release update in Anime Intelligence\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nA title you follow has a release update. Sign in to Anime Intelligence to review your release alerts or manage your email preference.\r\n",
		s.from, recipient, time.Now().UTC().Format(time.RFC1123Z))
	return s.transport.Send(ctx, recipient, []byte(message))
}

func runReleaseEmail(ctx context.Context, db *postgres.DB, sender *releaseEmailSender, logger *slog.Logger) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			claimCtx, cancelClaim := context.WithTimeout(ctx, 3*time.Second)
			claim, err := db.ClaimReleaseEmail(claimCtx)
			cancelClaim()
			if err != nil {
				logger.Warn("release email claim failed")
				continue
			}
			if claim == nil {
				continue
			}
			sendCtx, cancelSend := context.WithTimeout(ctx, 12*time.Second)
			delivered := sender.send(sendCtx, claim.Recipient) == nil
			cancelSend()
			finishCtx, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			err = db.FinishReleaseEmail(finishCtx, claim, delivered)
			cancelFinish()
			if err != nil {
				logger.Warn("release email completion failed")
			}
		}
	}
}
