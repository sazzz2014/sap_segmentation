package importer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"sap_segmentation/model"
	"testing"
	"time"
)

type fakeERP struct {
	offsets []int
	pages   [][]model.Segmentation
	err     error
}

func (f *fakeERP) FetchPage(_ context.Context, limit, offset int) ([]model.Segmentation, error) {
	f.offsets = append(f.offsets, offset)
	if f.err != nil {
		return nil, f.err
	}
	i := len(f.offsets) - 1
	if i >= len(f.pages) {
		return nil, nil
	}
	return f.pages[i], nil
}

type fakeRepo struct {
	calls  int
	err    error
	cancel context.CancelFunc
}

func (f *fakeRepo) UpsertBatch(_ context.Context, _ []model.Segmentation) error {
	f.calls++
	if f.cancel != nil {
		f.cancel()
	}
	return f.err
}
func service(c *fakeERP, r *fakeRepo) Importer {
	return Importer{Client: c, Repository: r, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), BatchSize: 50}
}

func TestPaginationUntilEmpty(t *testing.T) {
	row := []model.Segmentation{{AddressSAPID: "x"}}
	c := &fakeERP{pages: [][]model.Segmentation{row, row}}
	r := &fakeRepo{}
	s := service(c, r)
	stats, err := s.Run(context.Background())
	if err != nil || stats.Pages != 2 || stats.Records != 2 || r.calls != 2 || !reflect.DeepEqual(c.offsets, []int{0, 50, 100}) {
		t.Fatalf("stats=%v offsets=%v err=%v", stats, c.offsets, err)
	}
	c = &fakeERP{pages: [][]model.Segmentation{row}}
	s = service(c, r)
	s.OffsetStart = 1
	if _, err := s.Run(context.Background()); err != nil || !reflect.DeepEqual(c.offsets, []int{1, 51}) {
		t.Fatal("one based offsets")
	}
}

func TestFailuresStopImport(t *testing.T) {
	boom := errors.New("failure")
	c := &fakeERP{err: boom}
	r := &fakeRepo{}
	s := service(c, r)
	if _, err := s.Run(context.Background()); !errors.Is(err, boom) || r.calls != 0 {
		t.Fatal("ERP failure not propagated")
	}
	c = &fakeERP{pages: [][]model.Segmentation{{{AddressSAPID: "x"}}}}
	r = &fakeRepo{err: boom}
	s = service(c, r)
	stats, err := s.Run(context.Background())
	if !errors.Is(err, boom) || stats.Pages != 0 || len(c.offsets) != 1 {
		t.Fatal("DB failure advanced pagination")
	}
}

func TestIntervalCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &fakeERP{pages: [][]model.Segmentation{{{AddressSAPID: "x"}}}}
	r := &fakeRepo{cancel: cancel}
	s := service(c, r)
	s.Interval = time.Hour
	start := time.Now()
	stats, err := s.Run(ctx)
	if !errors.Is(err, context.Canceled) || time.Since(start) > time.Second || stats.Pages != 1 || len(c.offsets) != 1 {
		t.Fatalf("cancel failed: %v", err)
	}
}

func TestIntervalBetweenRequests(t *testing.T) {
	c := &fakeERP{pages: [][]model.Segmentation{{{AddressSAPID: "x"}}}}
	r := &fakeRepo{}
	s := service(c, r)
	s.Interval = 20 * time.Millisecond
	start := time.Now()
	if _, err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < s.Interval {
		t.Fatal("interval skipped")
	}
	c = &fakeERP{}
	s = service(c, r)
	s.Interval = time.Hour
	if _, err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
}
