package logging

import (
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type RedactingWriter struct {
	writer  io.Writer
	secrets []string
}

func (w RedactingWriter) Write(p []byte) (int, error) {
	s := string(p)
	for _, secret := range w.secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[REDACTED]")
		}
	}
	_, err := io.WriteString(w.writer, s)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
func Open(dir string, age time.Duration, stdout io.Writer, secrets ...string) (*slog.Logger, *os.File, error) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, nil, err
	}
	if err := Cleanup(dir, age, time.Now()); err != nil {
		return nil, nil, err
	}
	path := filepath.Join(dir, "segmentation_import.log")
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, nil, errors.New("current log must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, nil, err
	}

	variants := append([]string{}, secrets...)
	for _, s := range secrets {
		if s != "" {
			variants = append(variants, base64.StdEncoding.EncodeToString([]byte(s)))
			variants = append(variants, strings.Trim(s, "\""))
			variants = append(variants,
				url.QueryEscape(s),
				url.PathEscape(s),
				strings.Trim(strconv.Quote(s), "\""),
			)
		}
	}
	w := RedactingWriter{writer: io.MultiWriter(stdout, f), secrets: variants}
	return slog.New(slog.NewTextHandler(w, nil)), f, nil
}
