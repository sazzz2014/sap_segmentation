package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"sap_segmentation/internal/config"
	"sap_segmentation/internal/erp"
	"sap_segmentation/internal/importer"
	"sap_segmentation/internal/logging"
	"sap_segmentation/internal/storage/postgres"
)

func Run(ctx context.Context, stdout, stderr io.Writer) int {
	c, err := config.Load()
	if err != nil {
		fmt.Fprintln(stderr, "configuration:", err)
		return 1
	}
	maxLogAge := time.Duration(c.LogMaxAgeDays) * 24 * time.Hour
	logger, file, err := logging.Open(c.LogDir, maxLogAge, stdout, c.ConnAuth, c.DBPassword)
	if err != nil {
		slog.New(slog.NewTextHandler(stderr, nil)).Error("prepare logging failed", "error", err)
		return 1
	}
	defer file.Close()
	dbctx, cancel := context.WithTimeout(ctx, c.Timeout())
	repo, err := postgres.Open(dbctx, c.DSN())
	cancel()
	if err != nil {
		logger.Error("database connection failed", "operation", "db.connect", "error", err)
		return 1
	}
	defer repo.Close()
	client, err := erp.New(c.ConnURI, c.ConnAuth, c.UserAgent, c.Timeout(), logger)
	if err != nil {
		logger.Error("ERP client setup failed", "error", err)
		return 1
	}
	service := importer.Importer{
		Client:      client,
		Repository:  repo,
		Logger:      logger,
		BatchSize:   c.BatchSize,
		OffsetStart: c.OffsetStart,
		Interval:    time.Duration(c.Interval),
	}
	if _, err := service.Run(ctx); err != nil {
		logger.Error("import stopped", "operation", "import.run", "error", err)
		return 1
	}
	return 0
}
