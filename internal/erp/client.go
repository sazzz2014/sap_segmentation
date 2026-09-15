package erp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"sap_segmentation/model"
)

const maxResponseBytes = 8 << 20

type Client struct {
	endpoint  *url.URL
	auth      string
	userAgent string
	http      *http.Client
	log       *slog.Logger
}

func New(endpoint, auth, userAgent string, timeout time.Duration, logger *slog.Logger) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return nil, errors.New("invalid ERP endpoint")
	}
	httpClient := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &Client{
		endpoint:  u,
		auth:      "Basic " + base64.StdEncoding.EncodeToString([]byte(auth)),
		userAgent: userAgent,
		http:      httpClient,
		log:       logger,
	}, nil
}

func (c *Client) FetchPage(ctx context.Context, limit, offset int) (items []model.Segmentation, err error) {
	u := *c.endpoint
	q := u.Query()
	q.Set("p_limit", strconv.Itoa(limit))
	q.Set("p_offset", strconv.Itoa(offset))
	u.RawQuery = q.Encode()
	logURL := u
	logQuery := logURL.Query()
	for k := range logQuery {
		if k != "p_limit" && k != "p_offset" {
			logQuery.Set(k, "[REDACTED]")
		}
	}
	logURL.RawQuery = logQuery.Encode()
	c.log.InfoContext(ctx, "request ERP page", "operation", "erp.fetch", "endpoint", logURL.String(), "offset", offset, "batch_size", limit)
	defer func() {
		if err != nil {
			c.log.ErrorContext(ctx, "ERP page failed", "operation", "erp.fetch", "endpoint", logURL.String(), "offset", offset, "error", err)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.New("cannot create ERP request")
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("ERP transport: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ERP HTTP status %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read ERP response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, errors.New("ERP response exceeds 8 MiB")
	}
	return decodePage(body)
}
