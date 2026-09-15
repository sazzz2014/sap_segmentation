package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"sap_segmentation/model"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		client := http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:8080/health")
		if err != nil {
			os.Exit(1)
		}
		resp.Body.Close()
		if resp.StatusCode != 204 {
			os.Exit(1)
		}
		return
	}
	var mode atomic.Value
	mode.Store("initial")
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/mode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		switch r.URL.Query().Get("value") {
		case "initial", "updated", "malformed", "error", "slow":
			mode.Store(r.URL.Query().Get("value"))
			w.WriteHeader(204)
		default:
			http.Error(w, "invalid mode", 400)
		}
	})
	mux.HandleFunc("/segmentation", func(w http.ResponseWriter, r *http.Request) {
		login, pwd, ok := r.BasicAuth()
		if !ok || login != "mock" || pwd != "mock" || r.UserAgent() != "spacecount-test" {
			http.Error(w, "invalid auth or User-Agent", 401)
			return
		}
		limit, e1 := strconv.Atoi(r.URL.Query().Get("p_limit"))
		offset, e2 := strconv.Atoi(r.URL.Query().Get("p_offset"))
		if e1 != nil || e2 != nil || limit <= 0 || offset < 0 {
			http.Error(w, "invalid pagination", 400)
			return
		}
		switch mode.Load().(string) {
		case "malformed":
			w.Write([]byte("{invalid"))
			return
		case "error":
			http.Error(w, "fixture error", 500)
			return
		case "slow":
			select {
			case <-r.Context().Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
		rows := []model.Segmentation{{AddressSAPID: "SAP-001", AdrSegment: "A", SegmentID: 10}, {AddressSAPID: "SAP-002", AdrSegment: "B", SegmentID: 20}, {AddressSAPID: "SAP-003", AdrSegment: "C", SegmentID: 30}}
		if mode.Load().(string) == "updated" {
			rows[0].AdrSegment = "UPDATED"
			rows[0].SegmentID = 99
		}
		if offset >= len(rows) {
			rows = nil
		} else {
			end := min(offset+min(limit, len(rows)), len(rows))
			rows = rows[offset:end]
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Items []model.Segmentation `json:"items"`
		}{Items: rows})
	})
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
