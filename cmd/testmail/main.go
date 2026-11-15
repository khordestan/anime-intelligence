// testmail is an explicitly enabled, loopback-only synthetic CI fixture. It is
// not linked into the API image and never contacts an external mail provider.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/khordestan/anime-intelligence/internal/testutil/mailbox"
)

func main() {
	if os.Getenv("IDENTITY_MAIL_TEST") != "1" || os.Getenv("MAIL_SMTP_CA_FILE") == "" {
		slog.Error("test mail fixture requires explicit configuration")
		os.Exit(1)
	}
	server, err := mailbox.NewWithCAFile("127.0.0.1:8465", "127.0.0.1:8026", os.Getenv("MAIL_SMTP_USER"), os.Getenv("MAIL_SMTP_PASSWORD"), os.Getenv("MAIL_TEST_ACCESS_KEY"), os.Getenv("MAIL_SMTP_CA_FILE"))
	if err != nil {
		slog.Error("test mail fixture failed to start")
		os.Exit(1)
	}
	defer server.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}
