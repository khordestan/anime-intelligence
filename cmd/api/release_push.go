package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/khordestan/anime-intelligence/internal/httpapi"
	"github.com/khordestan/anime-intelligence/internal/platform/postgres"
	"github.com/khordestan/anime-intelligence/internal/platform/releasepush"
)

func releasePushFromEnv(getenv func(string) string, postgresql bool) (*releasepush.Sender, error) {
	switch getenv("RELEASE_PUSH_ENABLED") {
	case "", "0":
		return nil, nil
	case "1":
		if postgresql {
			return releasepush.New(releasepush.Config{
				PublicKey:  getenv("RELEASE_PUSH_PUBLIC_KEY"),
				PrivateKey: getenv("RELEASE_PUSH_PRIVATE_KEY"),
				Contact:    getenv("RELEASE_PUSH_CONTACT"),
				StorageKey: getenv("RELEASE_PUSH_STORAGE_KEY"),
			})
		}
	}
	return nil, releasepush.ErrConfiguration
}

func runReleasePush(ctx context.Context, db *postgres.DB, sender *releasepush.Sender, logger *slog.Logger, metrics *httpapi.RequestMetrics) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			claimCtx, cancelClaim := context.WithTimeout(ctx, 3*time.Second)
			claim, err := db.ClaimPush(claimCtx)
			if err != nil {
				cancelClaim()
				metrics.RecordPush(httpapi.PushClaimError)
				logger.Warn("browser push claim failed")
				continue
			}
			if claim == nil {
				cancelClaim()
				continue
			}
			current, err := db.PushClaimCurrent(claimCtx, claim)
			cancelClaim()
			if err != nil {
				metrics.RecordPush(httpapi.PushRecheckError)
				logger.Warn("browser push claim could not be rechecked")
				continue
			}
			if !current {
				continue
			}
			var outcome releasepush.Delivery
			var retry time.Duration
			subscription, err := claim.Subscription(sender)
			if err != nil {
				outcome = releasepush.Retry
				metrics.RecordPush(httpapi.PushSubscriptionError)
				logger.Warn("browser push subscription could not be opened")
			} else {
				outcome, retry = sender.Send(ctx, subscription)
				switch outcome {
				case releasepush.Delivered:
					metrics.RecordPush(httpapi.PushAccepted)
				case releasepush.Gone:
					metrics.RecordPush(httpapi.PushGone)
				case releasepush.Retry:
					metrics.RecordPush(httpapi.PushRetry)
				}
			}
			finishCtx, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			err = db.FinishPush(finishCtx, claim, outcome, retry)
			cancelFinish()
			if err != nil {
				metrics.RecordPush(httpapi.PushFinishError)
				logger.Warn("browser push completion failed")
			}
		}
	}
}
