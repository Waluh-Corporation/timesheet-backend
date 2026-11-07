package services

import (
	"errors"
)

// PDFMagicHeader is the standard byte sequence that all valid PDF documents begin with.
var PDFMagicHeader = []byte("%PDF-")

// ErrPDFGeneration indicates a failure during PDF document construction or rendering.
var ErrPDFGeneration = errors.New("pdf generation failed")
