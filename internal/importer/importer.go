package importer

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"sap_segmentation/model"
)

type ERPClient interface {
	FetchPage(context.Context, int, int) ([]model.Segmentation, error)
}

type Repository interface {
	UpsertBatch(context.Context, []model.Segmentation) error
}

type Stats struct {
	Pages   int
	Records int
}

type Importer struct {
	Client      ERPClient
	Repository  Repository
	Logger      *slog.Logger
	BatchSize   int
	OffsetStart int
	Interval    time.Duration
}

func (s Importer) Run(ctx context.Context) (stats Stats, err error) {
	if s.BatchSize <= 0 || s.OffsetStart < 0 || s.Interval < 0 {
		return stats, fmt.Errorf("invalid import settings")
	}
	for offset := s.OffsetStart; ; {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		start := time.Now()
		items, err := s.Client.FetchPage(ctx, s.BatchSize, offset)
		if err != nil {
			s.Logger.ErrorContext(ctx, "fetch failed", "operation", "import.fetch", "offset", offset, "error", err)
			return stats, fmt.Errorf("fetch offset %d: %w", offset, err)
		}
		if len(items) == 0 {
			s.Logger.InfoContext(ctx, "import complete", "operation", "import.complete", "pages", stats.Pages, "records", stats.Records)
			return stats, nil
		}
		if err := s.Repository.UpsertBatch(ctx, items); err != nil {
			s.Logger.ErrorContext(ctx, "save failed", "operation", "import.save", "offset", offset, "error", err)
			return stats, fmt.Errorf("save offset %d: %w", offset, err)
		}
		stats.Pages++
		stats.Records += len(items)
		s.Logger.InfoContext(ctx, "page committed", "operation", "import.save", "offset", offset, "received_count", len(items), "upserted_count", len(items), "duration", time.Since(start))
		if offset > math.MaxInt-s.BatchSize {
			return stats, fmt.Errorf("pagination offset overflow")
		}
		offset += s.BatchSize
		if s.Interval > 0 {
			timer := time.NewTimer(s.Interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return stats, ctx.Err()
			case <-timer.C:
			}
		}
	}
}
