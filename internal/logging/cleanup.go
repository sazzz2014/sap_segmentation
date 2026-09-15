package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func Cleanup(dir string, maxAge time.Duration, now time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read log directory: %w", err)
	}
	cutoff := now.Add(-maxAge)
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect log file: %w", err)
		}
		if info.Mode().IsRegular() && info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				return fmt.Errorf("remove expired log: %w", err)
			}
		}
	}
	return nil
}
