package services_test

import (
	"bytes"
	"image/png"
	"testing"

	"timesheet-backend/services"
)

func TestGenerateQRCodePNG(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		size        int
		wantErr     bool
		expectedErr string
	}{
		{
			name:        "empty content returns error",
			content:     "",
			size:        256,
			wantErr:     true,
			expectedErr: "qr code content cannot be empty",
		},
		{
			name:    "valid content with default size fallback",
			content: "https://timesheet.example.com/verify/doc/sample-uuid",
			size:    0,
			wantErr: false,
		},
		{
			name:    "valid content with explicit size",
			content: "https://timesheet.example.com/verify/doc/sample-uuid-12345",
			size:    128,
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			imgBytes, err := services.GenerateQRCodePNG(tc.content, tc.size)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.expectedErr)
				}
				if tc.expectedErr != "" && !bytes.Contains([]byte(err.Error()), []byte(tc.expectedErr)) {
					t.Errorf("expected error %q, got %v", tc.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(imgBytes) == 0 {
				t.Fatal("expected non-empty png bytes")
			}

			// Validate that the returned bytes form a valid PNG image
			cfg, err := png.DecodeConfig(bytes.NewReader(imgBytes))
			if err != nil {
				t.Fatalf("failed to decode generated png: %v", err)
			}

			expectedDim := tc.size
			if expectedDim <= 0 {
				expectedDim = 256
			}
			if cfg.Width != expectedDim || cfg.Height != expectedDim {
				t.Errorf("expected png dimensions %dx%d, got %dx%d", expectedDim, expectedDim, cfg.Width, cfg.Height)
			}
		})
	}
}
