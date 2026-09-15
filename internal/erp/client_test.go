package erp

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const validRow = `{"address_sap_id":"SAP-1","adr_segment":"A","segment_id":42}`

func TestPageShapes(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		count      int
		bad        bool
	}{
		{"array", "[" + validRow + "]", 1, false}, {"envelope", `{"items":[` + validRow + `],"hasMore":true}`, 1, false},
		{"empty", "", 0, false}, {"whitespace", " \n\t", 0, false}, {"null", "null", 0, false}, {"empty array", "[]", 0, false}, {"empty items", `{"items":[]}`, 0, false}, {"null items", `{"items":null}`, 0, false},
		{"malformed", "{invalid", 0, true}, {"missing items", "{}", 0, true}, {"wrong items", `{"items":{}}`, 0, true}, {"missing field", `[{"address_sap_id":"x","adr_segment":"A"}]`, 0, true}, {"null field", `[{"address_sap_id":"x","adr_segment":null,"segment_id":1}]`, 0, true},
		{"wrong type", `[{"address_sap_id":"x","adr_segment":"A","segment_id":"1"}]`, 0, true}, {"trailing data", "[] []", 0, true}, {"long segment", `[{"address_sap_id":"x","adr_segment":"12345678901234567","segment_id":1}]`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := decodePage([]byte(tc.body))
			if (err != nil) != tc.bad || len(rows) != tc.count {
				t.Fatalf("count=%d err=%v", len(rows), err)
			}
		})
	}
}

func TestRequestContractAndSafeLogs(t *testing.T) {
	var logs bytes.Buffer
	auth := "login:pwd:with:colon"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("p_limit") != "50" || r.URL.Query().Get("p_offset") != "100" || r.URL.Query().Get("token") != "secret-token" || r.URL.Query().Get("label") != "a & b" {
			t.Error("incorrect query")
		}
		if r.Header.Get("Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte(auth)) || r.UserAgent() != "agent" {
			t.Error("incorrect headers")
		}
		io.WriteString(w, "["+validRow+"]")
	}))
	defer srv.Close()
	c, err := New(srv.URL+"?token=secret-token&label=a%20%26%20b", auth, "agent", time.Second, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := c.FetchPage(context.Background(), 50, 100)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows %v err %v", rows, err)
	}
	if strings.Contains(logs.String(), auth) || strings.Contains(logs.String(), "secret-token") || !strings.Contains(logs.String(), "endpoint=") {
		t.Fatal("unsafe or incomplete log")
	}
}

func TestHTTPStatuses(t *testing.T) {
	for _, status := range []int{200, 204, 400, 401, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				if status != 204 {
					io.WriteString(w, "[]")
				}
			}))
			defer srv.Close()
			c, _ := New(srv.URL, "a:b", "agent", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
			_, err := c.FetchPage(context.Background(), 50, 0)
			if (err != nil) != (status >= 300) {
				t.Fatalf("status %d err %v", status, err)
			}
		})
	}
}

func TestTimeoutAndCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	c, _ := New(srv.URL, "a:b", "agent", 30*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := c.FetchPage(context.Background(), 1, 0); err == nil {
		t.Fatal("timeout ignored")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.FetchPage(ctx, 1, 0); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestRedirectDoesNotForwardCredentials(t *testing.T) {
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer srv.Close()
	c, _ := New(srv.URL, "a:b", "agent", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := c.FetchPage(context.Background(), 1, 0); err == nil || called {
		t.Fatal("redirect followed")
	}
}

func TestResponseLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strings.Repeat(" ", maxResponseBytes+1))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "a:b", "agent", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := c.FetchPage(context.Background(), 1, 0); err == nil {
		t.Fatal("oversized response accepted")
	}
}
