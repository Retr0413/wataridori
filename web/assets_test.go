package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesEmbeddedAssets(t *testing.T) {
	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/", contentType: "text/html", contains: "Cloud Run Deployments"},
		{path: "/dist/main.js", contentType: "text/javascript", contains: "loadStatus"},
		{path: "/styles.css", contentType: "text/css", contains: "--accent"},
	}

	handler := Handler()
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Header().Get("Content-Type"); !strings.Contains(got, tt.contentType) {
				t.Errorf("content-type = %q, want %q", got, tt.contentType)
			}
			if !strings.Contains(rec.Body.String(), tt.contains) {
				t.Errorf("body does not contain %q", tt.contains)
			}
		})
	}
}
