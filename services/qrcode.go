package services

import (
	"fmt"

	"github.com/skip2/go-qrcode"
)

// GenerateQRCodePNG creates a PNG image of a QR code encoding the provided content URL or text.
// size specifies the image width/height in pixels (e.g. 256).
func GenerateQRCodePNG(content string, size int) ([]byte, error) {
	if content == "" {
		return nil, fmt.Errorf("qr code content cannot be empty")
	}
	if size <= 0 {
		size = 256
	}
	pngBytes, err := qrcode.Encode(content, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("failed to generate qr code: %w", err)
	}
	return pngBytes, nil
}
