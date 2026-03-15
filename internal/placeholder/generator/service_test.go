package generator

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"testing"
)

func TestGeneratePNGDefaults(t *testing.T) {
	t.Parallel()

	svc := NewService()
	res, err := svc.Generate(Request{Width: 1200, Height: 628})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if res.Format != "png" {
		t.Fatalf("expected format png, got %s", res.Format)
	}
	if res.MIME != "image/png" {
		t.Fatalf("expected image/png mime, got %s", res.MIME)
	}
	if res.Text != "1200x628" {
		t.Fatalf("unexpected default text: %s", res.Text)
	}
	if len(res.Output) == 0 {
		t.Fatal("output must not be empty")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(res.Output))
	if err != nil {
		t.Fatalf("failed to decode generated image: %v", err)
	}
	if format != "png" {
		t.Fatalf("expected png config format, got %s", format)
	}
	if cfg.Width != 1200 || cfg.Height != 628 {
		t.Fatalf("unexpected dimensions %dx%d", cfg.Width, cfg.Height)
	}
}

func TestGenerateJPG(t *testing.T) {
	t.Parallel()

	svc := NewService()
	res, err := svc.Generate(Request{
		Width:      320,
		Height:     240,
		Format:     "jpg",
		Background: "112233",
		Foreground: "ffffff",
		Text:       "preview",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if res.MIME != "image/jpeg" {
		t.Fatalf("expected image/jpeg mime, got %s", res.MIME)
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(res.Output))
	if err != nil {
		t.Fatalf("failed to decode generated image: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("expected jpeg config format, got %s", format)
	}
	if cfg.Width != 320 || cfg.Height != 240 {
		t.Fatalf("unexpected dimensions %dx%d", cfg.Width, cfg.Height)
	}
}

func TestGenerateValidation(t *testing.T) {
	t.Parallel()

	svc := NewService()

	_, err := svc.Generate(Request{Width: 0, Height: 120})
	if err != ErrInvalidDimensions {
		t.Fatalf("expected ErrInvalidDimensions, got %v", err)
	}

	_, err = svc.Generate(Request{Width: 100, Height: 100, Format: "gif"})
	if err != ErrInvalidFormat {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}

	_, err = svc.Generate(Request{Width: 100, Height: 100, Background: "xyz"})
	if err != ErrInvalidColor {
		t.Fatalf("expected ErrInvalidColor, got %v", err)
	}
}
