package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"sap_segmentation/model"
)

type Repository struct {
	DB *sqlx.DB
}

func Open(ctx context.Context, dsn string) (*Repository, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return &Repository{DB: db}, nil
}
func (r *Repository) Close() error {
	return r.DB.Close()
}

const upsert = `INSERT INTO segmentation (address_sap_id, adr_segment, segment_id)
VALUES (:address_sap_id, :adr_segment, :segment_id)
ON CONFLICT (address_sap_id) DO UPDATE SET
adr_segment = EXCLUDED.adr_segment, segment_id = EXCLUDED.segment_id`

func (r *Repository) UpsertBatch(ctx context.Context, items []model.Segmentation) error {
	if len(items) == 0 {
		return nil
	}
	for i, item := range items {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("record %d: %w", i, err)
		}
	}
	tx, err := r.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin upsert: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareNamedContext(ctx, upsert)
	if err != nil {
		return fmt.Errorf("prepare upsert: %w", err)
	}
	defer stmt.Close()
	for i, item := range items {
		if _, err := stmt.ExecContext(ctx, item); err != nil {
			return fmt.Errorf("upsert record %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upsert: %w", err)
	}
	return nil
}
