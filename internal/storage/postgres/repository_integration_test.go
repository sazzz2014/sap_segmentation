//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"sap_segmentation/model"
	"strings"
	"testing"
	"time"
)

func TestPostgresContract(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL required: use scripts/local-check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	// Keep all DDL and data isolated on a single pooled connection. The shared
	// Compose database and its imported records are untouched by these tests.
	r.DB.SetMaxOpenConns(1)
	r.DB.SetMaxIdleConns(1)
	migration, err := os.ReadFile("../../../setup/install.sql")
	if err != nil {
		t.Fatal(err)
	}
	defer r.DB.ExecContext(context.Background(), "DROP TABLE IF EXISTS pg_temp.segmentation")
	// Verify the actual migration in a session-local schema, rather than a
	// CREATE IF NOT EXISTS against an already migrated table.
	localSQL := strings.Replace(string(migration), "CREATE TABLE IF NOT EXISTS segmentation", "CREATE TEMP TABLE segmentation", 1)
	if _, err := r.DB.ExecContext(ctx, localSQL); err != nil {
		t.Fatal(err)
	}
	var columns []struct {
		Name     string `db:"column_name"`
		Type     string `db:"data_type"`
		Nullable string `db:"is_nullable"`
		Length   *int   `db:"character_maximum_length"`
	}
	if err := r.DB.SelectContext(ctx, &columns, `SELECT column_name,data_type,is_nullable,character_maximum_length FROM information_schema.columns WHERE table_schema LIKE 'pg_temp_%' AND table_name='segmentation' ORDER BY ordinal_position`); err != nil {
		t.Fatal(err)
	}
	if len(columns) != 4 || columns[0].Type != "bigint" || columns[1].Length == nil || *columns[1].Length != 255 || columns[2].Length == nil || *columns[2].Length != 16 || columns[3].Type != "bigint" {
		t.Fatalf("incorrect schema: %+v", columns)
	}
	for _, c := range columns {
		if c.Nullable != "NO" {
			t.Fatalf("nullable column %s", c.Name)
		}
	}
	a := model.Segmentation{AddressSAPID: "SAP-1", AdrSegment: "A", SegmentID: 10}
	b := model.Segmentation{AddressSAPID: "SAP-2", AdrSegment: "B", SegmentID: 20}
	if err := r.UpsertBatch(ctx, []model.Segmentation{a, b}); err != nil {
		t.Fatal(err)
	}
	var before, after model.Segmentation
	if err := r.DB.GetContext(ctx, &before, "SELECT * FROM segmentation WHERE address_sap_id='SAP-1'"); err != nil {
		t.Fatal(err)
	}
	a.AdrSegment = "UPDATED"
	a.SegmentID = 99
	if err := r.UpsertBatch(ctx, []model.Segmentation{a}); err != nil {
		t.Fatal(err)
	}
	if err := r.DB.GetContext(ctx, &after, "SELECT * FROM segmentation WHERE address_sap_id='SAP-1'"); err != nil {
		t.Fatal(err)
	}
	if before.ID != after.ID || after.SegmentID != 99 || after.AdrSegment != "UPDATED" {
		t.Fatal("upsert failed")
	}
	if _, err := r.DB.ExecContext(ctx, "INSERT INTO segmentation(address_sap_id,adr_segment,segment_id) VALUES('SAP-1','A',1)"); err == nil {
		t.Fatal("uniqueness missing")
	}
	a.SegmentID = 100
	b.SegmentID = 101
	first := a
	first.SegmentID = 102
	if err := r.UpsertBatch(ctx, []model.Segmentation{first, b, a}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := r.DB.GetContext(ctx, &count, "SELECT count(DISTINCT id) FROM segmentation"); err != nil || count != 2 {
		t.Fatal("duplicate or identity failure")
	}
	// The second row violates a DB-only constraint after the first was written.
	if _, err := r.DB.ExecContext(ctx, "ALTER TABLE segmentation ADD CHECK(segment_id>=0)"); err != nil {
		t.Fatal(err)
	}
	a.SegmentID = 200
	b.SegmentID = -1
	if err := r.UpsertBatch(ctx, []model.Segmentation{a, b}); err == nil {
		t.Fatal("expected SQL error")
	}
	if err := r.DB.GetContext(ctx, &after, "SELECT * FROM segmentation WHERE address_sap_id='SAP-1'"); err != nil || after.SegmentID != 100 {
		t.Fatal("batch not rolled back")
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if err := r.UpsertBatch(cancelled, []model.Segmentation{a}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not propagated: %v", err)
	}
}
