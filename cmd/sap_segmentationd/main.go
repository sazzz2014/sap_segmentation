package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"sap_segmentation/internal/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := app.Run(ctx, os.Stdout, os.Stderr)
	cancel()
	os.Exit(code)
}
