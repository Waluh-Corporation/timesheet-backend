package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // Register JPEG decoder for uploaded signature images
	"image/png"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"timesheet-backend/assets"
)

var (
	parsedSigFontOnce sync.Once
	parsedSigFont     *opentype.Font
	parsedSigFontErr  error
)

func loadSignatureFont() (*opentype.Font, error) {
	parsedSigFontOnce.Do(func() {
		if len(assets.SignatureFontTTF) == 0 {
			parsedSigFontErr = fmt.Errorf("embedded signature font is empty")
			return
		}
		parsedSigFont, parsedSigFontErr = opentype.Parse(assets.SignatureFontTTF)
	})
	return parsedSigFont, parsedSigFontErr
}

// FormatSignatureShortName abbreviates a multi-word full name into "FirstName LastName"
// (e.g. "Faisal Al Munawar Fathur Rahman" -> "Faisal Rahman") for aesthetic cursive typed signatures.
func FormatSignatureShortName(fullName string) string {
	words := strings.Fields(strings.TrimSpace(fullName))
	if len(words) == 0 {
		return ""
	}
	if len(words) <= 2 {
		return strings.Join(words, " ")
	}
	return words[0] + " " + words[len(words)-1]
}

// RenderTypedSignaturePNG renders a cursive handwritten-style signature image (like "Faisal Rahman")
// in high-resolution black ink on a 100% transparent PNG canvas, automatically cropped to its ink bounding box.
func RenderTypedSignaturePNG(text string) ([]byte, error) {
	cleanText := strings.TrimSpace(text)
	if cleanText == "" {
		return nil, fmt.Errorf("signature text cannot be empty")
	}

	fnt, err := loadSignatureFont()
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded signature font: %w", err)
	}

	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    96,
		DPI:     144,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create font face: %w", err)
	}
	defer func() { _ = face.Close() }()

	adv := font.MeasureString(face, cleanText)
	metrics := face.Metrics()

	textWidth := adv.Ceil()
	textHeight := (metrics.Ascent + metrics.Descent).Ceil()

	// Add generous padding for sweeping cursive ascenders/descenders and swashes before tight auto-cropping
	padX := 120
	padY := 80
	canvasW := textWidth + padX*2
	if canvasW < 400 {
		canvasW = 400
	}
	canvasH := textHeight + padY*2
	if canvasH < 200 {
		canvasH = 200
	}

	dst := image.NewNRGBA(image.Rect(0, 0, canvasW, canvasH))
	baselineY := padY + metrics.Ascent.Ceil()

	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.NRGBA{R: 0, G: 0, B: 0, A: 255}),
		Face: face,
		Dot:  fixed.P(padX, baselineY),
	}
	d.DrawString(cleanText)

	cropped := cropToInkBounds(dst, 16)
	var out bytes.Buffer
	if err := png.Encode(&out, cropped); err != nil {
		return nil, fmt.Errorf("failed to encode rendered signature PNG: %w", err)
	}
	return out.Bytes(), nil
}

// DecodeDataURISignature extracts raw image bytes if the string is a base64 data URI (e.g. "data:image/png;base64,...").
func DecodeDataURISignature(val string) ([]byte, bool) {
	trimmed := strings.TrimSpace(val)
	if !strings.HasPrefix(trimmed, "data:image/") {
		return nil, false
	}
	idx := strings.Index(trimmed, ",")
	if idx == -1 || idx+1 >= len(trimmed) {
		return nil, false
	}
	rawB64 := trimmed[idx+1:]
	decoded, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return nil, false
	}
	return decoded, true
}

// NormalizeSignaturePNG decodes a raw PNG/JPEG signature image, strips white/near-white backgrounds
// into full alpha transparency with smooth anti-aliased edges, and crops whitespace around the ink.
func NormalizeSignaturePNG(rawBytes []byte) ([]byte, error) {
	if len(rawBytes) == 0 {
		return nil, fmt.Errorf("empty signature image bytes")
	}

	srcImg, _, err := image.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature image: %w", err)
	}

	bounds := srcImg.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid signature image dimensions")
	}

	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), srcImg, bounds.Min, draw.Src)

	// Convert white / near-white background pixels to transparent while preserving ink anti-aliasing
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := dst.NRGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			// Compute perceived luminance (0..255)
			lum := (299*uint32(c.R) + 587*uint32(c.G) + 114*uint32(c.B)) / 1000
			if lum >= 245 {
				// Pure white or very near-white background -> transparent
				dst.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
			} else if lum >= 200 {
				// Soft anti-aliased edge transition between 200..245
				alphaScale := float64(245-lum) / 45.0
				newAlpha := uint8(float64(c.A) * alphaScale)
				dst.SetNRGBA(x, y, color.NRGBA{R: c.R, G: c.G, B: c.B, A: newAlpha})
			}
		}
	}

	cropped := cropToInkBounds(dst, 12)
	var out bytes.Buffer
	if err := png.Encode(&out, cropped); err != nil {
		return nil, fmt.Errorf("failed to encode normalized signature PNG: %w", err)
	}
	return out.Bytes(), nil
}

// cropToInkBounds finds the bounding box of non-transparent pixels (alpha > 8) and returns a cropped image with margin.
func cropToInkBounds(img *image.NRGBA, margin int) *image.NRGBA {
	b := img.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	found := false

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.NRGBAAt(x, y).A > 8 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
				found = true
			}
		}
	}

	if !found {
		return img
	}

	minX -= margin
	if minX < b.Min.X {
		minX = b.Min.X
	}
	minY -= margin
	if minY < b.Min.Y {
		minY = b.Min.Y
	}
	maxX += margin + 1
	if maxX > b.Max.X {
		maxX = b.Max.X
	}
	maxY += margin + 1
	if maxY > b.Max.Y {
		maxY = b.Max.Y
	}

	cropped := image.NewNRGBA(image.Rect(0, 0, maxX-minX, maxY-minY))
	draw.Draw(cropped, cropped.Bounds(), img, image.Point{X: minX, Y: minY}, draw.Src)
	return cropped
}
