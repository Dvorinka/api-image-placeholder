package generator

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	maxDimension = 4096
	maxPixels    = 16_777_216 // 4096 * 4096
	maxTextLen   = 80
)

var (
	ErrInvalidDimensions = errors.New("width and height must be between 1 and 4096")
	ErrTooManyPixels     = errors.New("image dimensions are too large")
	ErrInvalidFormat     = errors.New("format must be png or jpg")
	ErrInvalidColor      = errors.New("color must be hex in #RRGGBB or #RGB format")
	ErrTextTooLong       = errors.New("text is too long")
)

type Request struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Format     string `json:"format"`
	Background string `json:"background"`
	Foreground string `json:"foreground"`
	Text       string `json:"text"`
}

type Result struct {
	Width      int
	Height     int
	Format     string
	MIME       string
	Background string
	Foreground string
	Text       string
	Output     []byte
	CacheKey   string
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Generate(req Request) (Result, error) {
	if req.Width < 1 || req.Width > maxDimension || req.Height < 1 || req.Height > maxDimension {
		return Result{}, ErrInvalidDimensions
	}
	if req.Width*req.Height > maxPixels {
		return Result{}, ErrTooManyPixels
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "png"
	}
	if format == "jpeg" {
		format = "jpg"
	}
	if format != "png" && format != "jpg" {
		return Result{}, ErrInvalidFormat
	}

	bgRaw := strings.TrimSpace(req.Background)
	if bgRaw == "" {
		bgRaw = "#d9dde3"
	}
	fgRaw := strings.TrimSpace(req.Foreground)
	if fgRaw == "" {
		fgRaw = "#2d3748"
	}

	bg, bgHex, err := parseHexColor(bgRaw)
	if err != nil {
		return Result{}, err
	}
	fg, fgHex, err := parseHexColor(fgRaw)
	if err != nil {
		return Result{}, err
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		text = fmt.Sprintf("%dx%d", req.Width, req.Height)
	}
	if len([]rune(text)) > maxTextLen {
		return Result{}, ErrTextTooLong
	}

	img := image.NewRGBA(image.Rect(0, 0, req.Width, req.Height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	drawCenteredText(img, text, fg)

	var out bytes.Buffer
	var mime string
	switch format {
	case "png":
		if err := png.Encode(&out, img); err != nil {
			return Result{}, fmt.Errorf("encode png: %w", err)
		}
		mime = "image/png"
	case "jpg":
		if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 90}); err != nil {
			return Result{}, fmt.Errorf("encode jpg: %w", err)
		}
		mime = "image/jpeg"
	}

	cacheKey := buildCacheKey(req.Width, req.Height, format, bgHex, fgHex, text)

	return Result{
		Width:      req.Width,
		Height:     req.Height,
		Format:     format,
		MIME:       mime,
		Background: bgHex,
		Foreground: fgHex,
		Text:       text,
		Output:     out.Bytes(),
		CacheKey:   cacheKey,
	}, nil
}

func drawCenteredText(dst draw.Image, text string, fg color.Color) {
	face := basicfont.Face7x13
	metrics := face.Metrics()

	textWidth := font.MeasureString(face, text).Round()
	textHeight := metrics.Height.Round()
	x := (dst.Bounds().Dx() - textWidth) / 2
	y := (dst.Bounds().Dy()-textHeight)/2 + metrics.Ascent.Round()

	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(fg),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func parseHexColor(input string) (color.RGBA, string, error) {
	raw := strings.TrimPrefix(strings.TrimSpace(strings.ToLower(input)), "#")
	if len(raw) == 3 {
		raw = strings.Repeat(string(raw[0]), 2) + strings.Repeat(string(raw[1]), 2) + strings.Repeat(string(raw[2]), 2)
	}
	if len(raw) != 6 {
		return color.RGBA{}, "", ErrInvalidColor
	}

	r, err := strconv.ParseUint(raw[0:2], 16, 8)
	if err != nil {
		return color.RGBA{}, "", ErrInvalidColor
	}
	g, err := strconv.ParseUint(raw[2:4], 16, 8)
	if err != nil {
		return color.RGBA{}, "", ErrInvalidColor
	}
	b, err := strconv.ParseUint(raw[4:6], 16, 8)
	if err != nil {
		return color.RGBA{}, "", ErrInvalidColor
	}

	hexValue := "#" + raw
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, hexValue, nil
}

func buildCacheKey(width, height int, format, bg, fg, text string) string {
	raw := fmt.Sprintf("%d|%d|%s|%s|%s|%s", width, height, format, bg, fg, text)
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}
