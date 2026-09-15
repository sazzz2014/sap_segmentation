package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sap_segmentation/internal/app"
	"sap_segmentation/internal/config"
	"sap_segmentation/internal/storage/postgres"
	"sap_segmentation/model"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "E2E FAILED:", err)
		os.Exit(1)
	}
	fmt.Println("E2E PASSED: initial import, stable IDs, updates, cleanup, dual logs, HTTP/JSON/timeout/DB errors")
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c, err := config.Load()
	if err != nil {
		return err
	}
	r, err := postgres.Open(ctx, c.DSN())
	if err != nil {
		return err
	}
	defer r.Close()
	var before []model.Segmentation
	if err := r.DB.SelectContext(ctx, &before, "SELECT * FROM segmentation ORDER BY address_sap_id"); err != nil {
		return err
	}
	if len(before) != 3 || before[0].AdrSegment != "A" || before[0].SegmentID != 10 || before[1].SegmentID != 20 || before[2].SegmentID != 30 {
		return fmt.Errorf("unexpected initial data: %v", before)
	}
	modeURL, err := url.Parse(c.ConnURI)
	if err != nil {
		return err
	}
	modeURL.Path = "/mode"
	setMode := func(mode string) error {
		u := *modeURL
		q := url.Values{}
		q.Set("value", mode)
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
		if err != nil {
			return err
		}
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 204 {
			return fmt.Errorf("mode change status %d", resp.StatusCode)
		}
		return nil
	}
	if err := setMode("updated"); err != nil {
		return err
	}
	expired := filepath.Join(c.LogDir, "expired-e2e.log")
	if err := os.WriteFile(expired, []byte("old log"), 0600); err != nil {
		return err
	}
	old := time.Now().Add(-time.Duration(c.LogMaxAgeDays+1) * 24 * time.Hour)
	if err := os.Chtimes(expired, old, old); err != nil {
		return err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := app.Run(ctx, io.MultiWriter(os.Stdout, &stdout), &stderr); code != 0 {
		return fmt.Errorf("reimport exit %d: %s", code, stderr.String())
	}
	var after []model.Segmentation
	if err := r.DB.SelectContext(ctx, &after, "SELECT * FROM segmentation ORDER BY address_sap_id"); err != nil {
		return err
	}
	if len(after) != 3 {
		return fmt.Errorf("reimport count %d", len(after))
	}
	for i := range before {
		if before[i].ID != after[i].ID {
			return fmt.Errorf("id changed for %s", before[i].AddressSAPID)
		}
	}
	if after[0].AdrSegment != "UPDATED" || after[0].SegmentID != 99 {
		return fmt.Errorf("update not applied")
	}
	if _, err := os.Stat(expired); !os.IsNotExist(err) {
		return fmt.Errorf("expired log was not removed")
	}
	logBody, err := os.ReadFile(filepath.Join(c.LogDir, "segmentation_import.log"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(logBody), "endpoint=") || !strings.Contains(stdout.String(), "endpoint=") {
		return fmt.Errorf("missing endpoint in console or file log")
	}
	for _, mode := range []string{"malformed", "error", "slow"} {
		if err := setMode(mode); err != nil {
			return err
		}
		os.Setenv("CONN_TIMEOUT", "1")
		stdout.Reset()
		if app.Run(ctx, &stdout, &stderr) == 0 || !strings.Contains(stdout.String(), "level=ERROR") {
			return fmt.Errorf("%s did not fail and log error", mode)
		}
	}
	if err := setMode("initial"); err != nil {
		return err
	}
	// Force a real SQL failure after the HTTP page has been read.
	if _, err := r.DB.ExecContext(ctx, "ALTER TABLE segmentation RENAME TO segmentation_e2e_hidden"); err != nil {
		return err
	}
	defer r.DB.ExecContext(context.Background(), "ALTER TABLE segmentation_e2e_hidden RENAME TO segmentation")
	stdout.Reset()
	if app.Run(ctx, &stdout, &stderr) == 0 || !strings.Contains(stdout.String(), "import.save") {
		return fmt.Errorf("DB error not propagated and logged")
	}
	return nil
}
