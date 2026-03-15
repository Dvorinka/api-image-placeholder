package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"apiservices/image-placeholder/internal/placeholder/generator"
)

type Handler struct {
	service *generator.Service
}

func NewHandler(service *generator.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/v1/placeholder/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/placeholder/"), "/")
	switch {
	case path == "generate":
		h.handleGenerate(w, r)
	case strings.HasPrefix(path, "image/"):
		h.handleImage(w, r, strings.TrimPrefix(path, "image/"))
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req generator.Request
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.service.Generate(req)
	if err != nil {
		handleGenerateError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"width":         result.Width,
			"height":        result.Height,
			"format":        result.Format,
			"output_mime":   result.MIME,
			"background":    result.Background,
			"foreground":    result.Foreground,
			"text":          result.Text,
			"cache_key":     result.CacheKey,
			"output_base64": base64.StdEncoding.EncodeToString(result.Output),
		},
	})
}

func (h *Handler) handleImage(w http.ResponseWriter, r *http.Request, pathSpec string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	width, height, format, err := parseSizeSpec(pathSpec)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if queryFormat := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("format"))); queryFormat != "" {
		format = queryFormat
	}

	result, err := h.service.Generate(generator.Request{
		Width:      width,
		Height:     height,
		Format:     format,
		Background: r.URL.Query().Get("bg"),
		Foreground: r.URL.Query().Get("fg"),
		Text:       r.URL.Query().Get("text"),
	})
	if err != nil {
		handleGenerateError(w, err)
		return
	}

	etag := "\"" + result.CacheKey + "\""
	if strings.TrimSpace(r.Header.Get("If-None-Match")) == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", result.MIME)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Placeholder-Dimensions", fmt.Sprintf("%dx%d", result.Width, result.Height))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(result.Output)
}

func parseSizeSpec(spec string) (int, int, string, error) {
	if strings.Contains(spec, "/") {
		return 0, 0, "", errors.New("invalid image spec")
	}

	format := "png"
	size := spec
	if idx := strings.LastIndex(spec, "."); idx >= 0 {
		format = strings.ToLower(strings.TrimSpace(spec[idx+1:]))
		size = spec[:idx]
	}

	parts := strings.Split(strings.ToLower(size), "x")
	if len(parts) != 2 {
		return 0, 0, "", errors.New("image path must be WIDTHxHEIGHT")
	}

	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, "", errors.New("invalid width")
	}
	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, "", errors.New("invalid height")
	}

	if format == "jpeg" {
		format = "jpg"
	}
	return width, height, format, nil
}

func handleGenerateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, generator.ErrInvalidDimensions),
		errors.Is(err, generator.ErrTooManyPixels),
		errors.Is(err, generator.ErrInvalidFormat),
		errors.Is(err, generator.ErrInvalidColor),
		errors.Is(err, generator.ErrTextTooLong):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "failed to generate image")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"failed to marshal response"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return errors.New("invalid json body")
	}

	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("json body must contain a single object")
	}
	return nil
}
