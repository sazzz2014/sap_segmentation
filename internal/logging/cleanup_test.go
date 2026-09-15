package logging

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCleanup(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Truncate(time.Second)
	age := 24 * time.Hour
	for _, tc := range []struct {
		name  string
		mtime time.Time
	}{{"old", now.Add(-age - time.Second)}, {"boundary", now.Add(-age)}, {"young", now}} {
		p := filepath.Join(dir, tc.name)
		if err := os.WriteFile(p, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, tc.mtime, tc.mtime); err != nil {
			t.Fatal(err)
		}
	}
	nested := filepath.Join(dir, "nested")
	os.Mkdir(nested, 0700)
	os.Chtimes(nested, now.Add(-2*age), now.Add(-2*age))
	target := filepath.Join(t.TempDir(), "target")
	os.WriteFile(target, []byte("outside"), 0600)
	os.Chtimes(target, now.Add(-2*age), now.Add(-2*age))
	link := filepath.Join(dir, "link")
	linkErr := os.Symlink(target, link)
	if err := Cleanup(dir, age, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Fatal("expired file retained")
	}
	for _, p := range []string{filepath.Join(dir, "boundary"), filepath.Join(dir, "young"), nested, target} {
		if _, err := os.Stat(p); err != nil {
			t.Fatal(err)
		}
	}
	if linkErr == nil {
		if _, err := os.Lstat(link); err != nil {
			t.Fatal("symlink removed")
		}
	} else {
		t.Log("symlink unavailable on host; Linux Docker test covers it")
	}
}

func TestDualLogsAndRedaction(t *testing.T) {
	dir := t.TempDir()
	var console bytes.Buffer
	secret := "login:secret"
	logger, f, err := Open(dir, 24*time.Hour, &console, secret)
	if err != nil {
		t.Fatal(err)
	}
	logger.Error("failed", "endpoint", "http://example.test", "error", secret+" "+base64.StdEncoding.EncodeToString([]byte(secret)))
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "segmentation_import.log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != console.String() || strings.Contains(string(body), secret) || strings.Contains(string(body), base64.StdEncoding.EncodeToString([]byte(secret))) || !strings.Contains(string(body), "endpoint=") {
		t.Fatalf("incorrect dual log: %s", body)
	}
}
