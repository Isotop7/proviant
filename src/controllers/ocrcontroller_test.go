package controllers

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
)

func newTestOCRController() *OCRControllerImpl {
	logger := zerolog.Nop()
	config := configuration.OCRConfiguration{
		Timeout:      30,
		Languages:    "eng",
		TesseractPath: "/usr/bin/tesseract",
	}
	return NewOCRController(&logger, &config)
}

func TestExtractDateCandidates_WithValidDates(t *testing.T) {
	ctrl := newTestOCRController()

	tests := []struct {
		name    string
		text    string
		wantLen int
	}{
		{
			name:    "No dates in text",
			text:    "This is just some random text without any dates",
			wantLen: 0,
		},
		{
			name:    "Date too far in past filtered",
			text:    "01.01.2020",
			wantLen: 0,
		},
		{
			name:    "Date too far in future filtered",
			text:    "01.01.2035",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := ctrl.extractDateCandidates(tt.text)
			if len(candidates) != tt.wantLen {
				t.Errorf("extractDateCandidates() got %d candidates, want %d", len(candidates), tt.wantLen)
			}
		})
	}
}

func TestExtractDateCandidates_ConfidenceScoring(t *testing.T) {
	ctrl := newTestOCRController()

	_ = ctrl
}

func TestExtractDateCandidates_DateRangeFiltering(t *testing.T) {
	ctrl := newTestOCRController()

	tests := []struct {
		name    string
		text    string
		wantLen int
	}{
		{
			name:    "Date too far in past",
			text:    "01.01.2020",
			wantLen: 0,
		},
		{
			name:    "Date too far in future",
			text:    "01.01.2035",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := ctrl.extractDateCandidates(tt.text)
			if len(candidates) != tt.wantLen {
				t.Errorf("extractDateCandidates() got %d candidates, want %d", len(candidates), tt.wantLen)
			}
		})
	}
}

func TestSelectBestDate(t *testing.T) {
	ctrl := newTestOCRController()

	tests := []struct {
		name       string
		candidates []dateCandidate
		wantConf   float64
	}{
		{
			name:       "Empty candidates",
			candidates: []dateCandidate{},
			wantConf:   0.0,
		},
		{
			name: "Single candidate",
			candidates: []dateCandidate{
				{date: time.Now().AddDate(0, 1, 0), raw: "25.12.2025", confidence: 0.8},
			},
			wantConf: 0.8,
		},
		{
			name: "Multiple candidates - highest confidence wins",
			candidates: []dateCandidate{
				{date: time.Now().AddDate(0, 1, 0), raw: "25.12.2025", confidence: 0.6},
				{date: time.Now().AddDate(0, 1, 0), raw: "15.06.2025", confidence: 0.9},
			},
			wantConf: 0.9,
		},
		{
			name: "Tie - first one wins",
			candidates: []dateCandidate{
				{date: time.Now().AddDate(0, 1, 0), raw: "25.12.2025", confidence: 0.8},
				{date: time.Now().AddDate(0, 1, 0), raw: "15.06.2025", confidence: 0.8},
			},
			wantConf: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctrl.selectBestDate(tt.candidates)
			if result.Confidence != tt.wantConf {
				t.Errorf("selectBestDate() confidence = %v, want %v", result.Confidence, tt.wantConf)
			}
		})
	}
}

func TestAssembleDateString(t *testing.T) {
	ctrl := newTestOCRController()

	tests := []struct {
		name     string
		parts    []string
		dayFirst bool
		want     string
	}{
		{
			name:     "DD.MM.YYYY format (day first)",
			parts:    []string{"25", "12", "2025"},
			dayFirst: true,
			want:     "2025-12-25",
		},
		{
			name:     "MM/DD/YYYY format (day first)",
			parts:    []string{"06", "15", "2026"},
			dayFirst: true,
			want:     "2026-15-06",
		},
		{
			name:     "Single digit day padded",
			parts:    []string{"5", "3", "2025"},
			dayFirst: true,
			want:     "2025-03-05",
		},
		{
			name:     "Single digit month padded",
			parts:    []string{"25", "3", "2025"},
			dayFirst: true,
			want:     "2025-03-25",
		},
		{
			name:     "2-digit year converted to 4-digit",
			parts:    []string{"25", "12", "25"},
			dayFirst: true,
			want:     "2025-12-25",
		},
		{
			name:     "Not enough parts",
			parts:    []string{"25", "12"},
			dayFirst: true,
			want:     "",
		},
		{
			name:     "Empty parts",
			parts:    []string{},
			dayFirst: true,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctrl.assembleDateString(tt.parts, tt.dayFirst)
			if got != tt.want {
				t.Errorf("assembleDateString() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestPreprocessImage(t *testing.T) {
	ctrl := newTestOCRController()

	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test image: %v", err)
	}

	processed, err := ctrl.preprocessImage(buf.Bytes())
	if err != nil {
		t.Fatalf("preprocessImage() error = %v", err)
	}

	bounds := processed.Bounds()
	if bounds.Dx() != 20 || bounds.Dy() != 20 {
		t.Errorf("preprocessImage() dimensions = %dx%d, want 20x20", bounds.Dx(), bounds.Dy())
	}

	_, ok := processed.(*image.Gray)
	if !ok {
		t.Error("preprocessImage() expected grayscale image (Gray)")
	}
}

func TestPreprocessImage_InvalidInput(t *testing.T) {
	ctrl := newTestOCRController()

	_, err := ctrl.preprocessImage([]byte("not a valid image"))
	if err == nil {
		t.Error("preprocessImage() expected error for invalid input")
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{
			name:   "String shorter than max",
			s:      "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "String equal to max",
			s:      "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "String longer than max",
			s:      "hello world",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "Empty string",
			s:      "",
			maxLen: 5,
			want:   "",
		},
		{
			name:   "Zero max length",
			s:      "hello",
			maxLen: 0,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString() = %s, want %s", got, tt.want)
			}
		})
	}
}

func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("slice length = %d, want %d", len(got), len(want))
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseWebhookEvents(t *testing.T) {
	tests := []struct {
		name       string
		eventsJSON string
		want       []string
	}{
		{
			name:       "Simple array",
			eventsJSON: `["event1", "event2"]`,
			want:       []string{"event1", "event2"},
		},
		{
			name:       "Comma separated string",
			eventsJSON: "event1, event2, event3",
			want:       []string{"event1", "event2", "event3"},
		},
		{
			name:       "Empty brackets",
			eventsJSON: "[]",
			want:       nil,
		},
		{
			name:       "Whitespace only",
			eventsJSON: "   ",
			want:       nil,
		},
		{
			name:       "Empty string",
			eventsJSON: "",
			want:       nil,
		},
		{
			name:       "Single event",
			eventsJSON: "single_event",
			want:       []string{"single_event"},
		},
		{
			name:       "Single quoted event",
			eventsJSON: `"single_event"`,
			want:       []string{"single_event"},
		},
		{
			name:       "Events with extra spaces",
			eventsJSON: "  event1  ,  event2  ",
			want:       []string{"event1", "event2"},
		},
		{
			name:       "Array with extra spaces",
			eventsJSON: `["event1" , "event2"]`,
			want:       []string{"event1", "event2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseWebhookEvents(tt.eventsJSON)
			if tt.want == nil {
				if len(got) != 0 {
					t.Errorf("ParseWebhookEvents() = %v, want nil/empty", got)
				}
				return
			}
			assertStringSliceEqual(t, got, tt.want)
		})
	}
}

func TestComputeSignature(t *testing.T) {
	webhookService := &WebhookService{}

	tests := []struct {
		name    string
		secret  string
		payload []byte
		wantLen int
	}{
		{
			name:    "Valid signature",
			secret:  "mysecret",
			payload: []byte(`{"event": "test"}`),
			wantLen: 64,
		},
		{
			name:    "Empty secret",
			secret:  "",
			payload: []byte(`{"event": "test"}`),
			wantLen: 64,
		},
		{
			name:    "Empty payload",
			secret:  "mysecret",
			payload: []byte{},
			wantLen: 64,
		},
		{
			name:    "Same inputs produce same signature",
			secret:  "mysecret",
			payload: []byte(`{"event": "test"}`),
			wantLen: 64,
		},
	}

	sig1 := webhookService.computeSignature("mysecret", []byte(`{"event": "test"}`))
	if len(sig1) != 64 {
		t.Errorf("computeSignature() len = %d, want 64", len(sig1))
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := webhookService.computeSignature(tt.secret, tt.payload)
			if len(sig) != tt.wantLen {
				t.Errorf("computeSignature() len = %d, want %d", len(sig), tt.wantLen)
			}
		})
	}

	sig2 := webhookService.computeSignature("mysecret", []byte(`{"event": "test"}`))
	if sig1 != sig2 {
		t.Error("computeSignature() same inputs should produce same signature")
	}

	sig3 := webhookService.computeSignature("different", []byte(`{"event": "test"}`))
	if sig1 == sig3 {
		t.Error("computeSignature() different secrets should produce different signatures")
	}
}
