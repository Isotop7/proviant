package api

// ExpiryScanResponse returns detected expiry date and confidence
type ExpiryScanResponse struct {
	DetectedDate string  `json:"detectedDate"`       // ISO 8601 (YYYY-MM-DD) if parsed, else empty
	Confidence   float64 `json:"confidence"`         // 0.0–1.0
	RawText      string  `json:"rawText"`            // full OCR text for manual correction
	Language     string  `json:"language,omitempty"` // detected language if available
}
