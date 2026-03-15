package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"apiservices/image-placeholder/internal/placeholder/generator"
)

func TestGenerateEndpoint(t *testing.T) {
	t.Parallel()

	h := NewHandler(generator.NewService())
	req := httptest.NewRequest(http.MethodPost, "/v1/placeholder/generate", strings.NewReader(`{"width":640,"height":360,"background":"#112233","foreground":"#ffffff"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var payload struct {
		Data struct {
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			Format       string `json:"format"`
			OutputMIME   string `json:"output_mime"`
			OutputBase64 string `json:"output_base64"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response json: %v", err)
	}

	if payload.Data.Width != 640 || payload.Data.Height != 360 {
		t.Fatalf("unexpected dimensions %dx%d", payload.Data.Width, payload.Data.Height)
	}
	if payload.Data.Format != "png" {
		t.Fatalf("expected png format, got %s", payload.Data.Format)
	}
	if payload.Data.OutputMIME != "image/png" {
		t.Fatalf("expected image/png, got %s", payload.Data.OutputMIME)
	}
	if payload.Data.OutputBase64 == "" {
		t.Fatal("output_base64 should not be empty")
	}
}

func TestImageEndpointETag(t *testing.T) {
	t.Parallel()

	h := NewHandler(generator.NewService())
	req := httptest.NewRequest(http.MethodGet, "/v1/placeholder/image/320x120.png?bg=000000&fg=ffffff&text=hero", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	etag := rr.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag header must be present")
	}
	if rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("expected image/png, got %s", rr.Header().Get("Content-Type"))
	}

	headReq := httptest.NewRequest(http.MethodHead, "/v1/placeholder/image/320x120.png?bg=000000&fg=ffffff&text=hero", nil)
	headRR := httptest.NewRecorder()
	h.ServeHTTP(headRR, headReq)
	if headRR.Code != http.StatusOK {
		t.Fatalf("expected HEAD 200, got %d", headRR.Code)
	}
	if headRR.Body.Len() != 0 {
		t.Fatalf("expected empty HEAD body, got %d bytes", headRR.Body.Len())
	}

	conditionalReq := httptest.NewRequest(http.MethodGet, "/v1/placeholder/image/320x120.png?bg=000000&fg=ffffff&text=hero", nil)
	conditionalReq.Header.Set("If-None-Match", etag)
	conditionalRR := httptest.NewRecorder()
	h.ServeHTTP(conditionalRR, conditionalReq)
	if conditionalRR.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", conditionalRR.Code)
	}
}
