package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunHealthcheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	if code := runHealthcheck(addr); code != 0 {
		t.Fatalf("expected exit code 0 for healthy server, got %d", code)
	}

	srv.Close()
	if code := runHealthcheck(addr); code != 1 {
		t.Fatalf("expected exit code 1 after server closed, got %d", code)
	}
}
