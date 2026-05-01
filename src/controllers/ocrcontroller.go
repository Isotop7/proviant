package controllers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
	"golang.org/x/image/draw"
)

// OCRController interfaces for scanning expiry dates from images
type OCRController interface {
	ScanExpiryDate(image []byte) (*api.ExpiryScanResponse, error)
}

type OCRControllerImpl struct {
	Logger *zerolog.Logger
	Config configuration.OCRConfiguration
}

// NewOCRController creates a new OCR controller instance
func NewOCRController(logger *zerolog.Logger, config *configuration.OCRConfiguration) *OCRControllerImpl {
	return &OCRControllerImpl{
		Logger: logger,
		Config: *config,
	}
}

// ScanExpiryDate processes an image and returns detected expiry date
func (c *OCRControllerImpl) ScanExpiryDate(image []byte) (*api.ExpiryScanResponse, error) {
	// 1. Preprocess: upscale 2x and convert to grayscale
	processed, err := c.preprocessImage(image)
	if err != nil {
		return nil, fmt.Errorf("image preprocessing failed: %w", err)
	}

	// 2. Encode to PNG
	encoded, err := c.encodeImage(processed)
	if err != nil {
		return nil, fmt.Errorf("image encoding failed: %w", err)
	}

	// 3. Run Tesseract with basic settings
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Config.Timeout)*time.Second)
	defer cancel()

	stdout, stderr, err := runTesseract(ctx, encoded, c.Config.Languages)
	if err != nil {
		return nil, fmt.Errorf("tesseract error: %w, stderr: %s", err, stderr)
	}

	rawText := strings.TrimSpace(stdout)

	// 4. Compute hash
	hash := sha256.Sum256(image)
	imageHash := hex.EncodeToString(hash[:])

	// 5. Extract date candidates
	candidates := c.extractDateCandidates(rawText)

	// 6. Select best date
	best := c.selectBestDate(candidates)

	// 7. Build response
	response := &api.ExpiryScanResponse{
		DetectedDate: best.DetectedDate,
		Confidence:   best.Confidence,
		RawText:      rawText, // always return full OCR text
	}

	// 8. Log
	c.Logger.Debug().
		Str("imageHash", imageHash).
		Str("rawText", rawText).
		Str("detectedDate", response.DetectedDate).
		Float64("confidence", response.Confidence).
		Msg("OCR scan completed")

	return response, nil
}

// preprocessImage: upscale 2x, grayscale only
func (c *OCRControllerImpl) preprocessImage(imgBytes []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	newBounds := image.Rect(0, 0, bounds.Dx()*2, bounds.Dy()*2)
	upscaled := image.NewRGBA(newBounds)
	draw.CatmullRom.Scale(upscaled, newBounds, img, bounds, draw.Over, nil)

	gray := image.NewGray(newBounds)
	for y := 0; y < newBounds.Dy(); y++ {
		for x := 0; x < newBounds.Dx(); x++ {
			c := upscaled.RGBAAt(x, y)
			lum := 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
			gray.SetGray(x, y, color.Gray{Y: uint8(lum)})
		}
	}
	return gray, nil
}

// encodeImage to PNG
func (c *OCRControllerImpl) encodeImage(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// runTesseract with basic parameters
func runTesseract(ctx context.Context, img []byte, langs string) (stdout, stderr string, err error) {
	// #nosec G204 — command is hardcoded, not user-controlled
	cmd := exec.CommandContext(ctx, "tesseract", "stdin", "stdout",
		"-l", langs,
		"--dpi", "300",
	)
	cmd.Stdin = bytes.NewReader(img)
	outBytes, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		return "", stderr, err
	}
	return string(outBytes), "", nil
}

type dateCandidate struct {
	date       time.Time
	raw        string
	confidence float64
}

// extractDateCandidates finds all plausible dates in OCR text and scores them
func (c *OCRControllerImpl) extractDateCandidates(text string) []dateCandidate {
	var candidates []dateCandidate
	now := time.Now()

	// Normalize whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// Keywords
	keywordRe := regexp.MustCompile(`(?i)(Mindesthaltbarkeit|Verbrauch[_\s]?bis|Haltbar[_\s]?bis|Mindestens[_\s]?haltbar[_\s]?bis|Zu[_\s]?verbrauchen[_\s]?bis|Gültig[_\s]?bis|Best[_\s]?before|Expiry|Use[_\s]?by|Mindestens|Haltbar|Verbrauchen|Mindesthaltbar)`)
	hasKeyword := keywordRe.MatchString(text)

	// Date patterns — match even if surrounded by other chars
	patterns := []struct {
		regex    *regexp.Regexp
		layout   string
		dayFirst bool
	}{
		{regexp.MustCompile(`(\d{1,2})\.(\d{1,2})\.(\d{2,4})`), "02.01.2006", true},
		{regexp.MustCompile(`(\d{1,2})/(\d{1,2})/(\d{2,4})`), "02/01/2006", true},
		{regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`), util.DefaultDateFormatParseStr, false},
	}

	for _, p := range patterns {
		matches := p.regex.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			raw := m[0]
			dateStr := c.assembleDateString(m[1:], p.dayFirst)
			t, err := time.Parse(p.layout, dateStr)
			if err != nil {
				continue
			}
			// Reasonable range
			if t.Before(now.AddDate(0, 0, -30)) || t.After(now.AddDate(5, 0, 0)) {
				continue
			}
			conf := 0.5
			if hasKeyword {
				conf += 0.3
				lowerText := strings.ToLower(text)
				kwIdx := keywordRe.FindStringIndex(lowerText)
				dateIdx := strings.Index(lowerText, strings.ToLower(raw))
				if kwIdx != nil && kwIdx[0] >= 0 && dateIdx >= 0 && abs(kwIdx[0]-dateIdx) <= 30 {
					conf += 0.1
				}
			}
			if p.dayFirst {
				conf += 0.1
			}
			if len(m[len(m)-1]) == 4 {
				conf += 0.1
			}
			if conf > 1.0 {
				conf = 1.0
			}
			candidates = append(candidates, dateCandidate{date: t, raw: raw, confidence: conf})
		}
	}
	return candidates
}

// assembleDateString reconstructs a YYYY-MM-DD date from regex capture groups
func (c *OCRControllerImpl) assembleDateString(parts []string, dayFirst bool) string {
	if len(parts) < 3 {
		return ""
	}
	day, month, year := parts[0], parts[1], parts[2]
	if !dayFirst {
		day, month = month, day
	}
	if len(day) == 1 {
		day = "0" + day
	}
	if len(month) == 1 {
		month = "0" + month
	}
	if len(year) == 2 {
		year = "20" + year
	}
	return fmt.Sprintf("%s-%s-%s", year, month, day)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// selectBestDate picks the highest confidence candidate
func (c *OCRControllerImpl) selectBestDate(candidates []dateCandidate) *api.ExpiryScanResponse {
	if len(candidates) == 0 {
		return &api.ExpiryScanResponse{Confidence: 0.0}
	}
	best := candidates[0]
	for _, cand := range candidates[1:] {
		if cand.confidence > best.confidence {
			best = cand
		}
	}
	return &api.ExpiryScanResponse{
		DetectedDate: best.date.Format(util.DefaultDateFormatParseStr),
		Confidence:   best.confidence,
		RawText:      best.raw,
	}
}
