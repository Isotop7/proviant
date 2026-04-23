package controllers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	"github.com/rs/zerolog"
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
func NewOCRController(logger *zerolog.Logger, config configuration.OCRConfiguration) *OCRControllerImpl {
	return &OCRControllerImpl{
		Logger: logger,
		Config: config,
	}
}

// ScanExpiryDate processes an image and returns detected expiry date
func (c *OCRControllerImpl) ScanExpiryDate(image []byte) (*api.ExpiryScanResponse, error) {
	// 1. Run Tesseract OCR via subprocess
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Config.Timeout)*time.Second)
	defer cancel()

	stdout, stderr, err := runTesseract(ctx, image, c.Config.Languages)
	if err != nil {
		return nil, fmt.Errorf("tesseract error: %w, stderr: %s", err, stderr)
	}

	rawText := strings.TrimSpace(stdout)

	// 2. Compute image hash for potential dedup/logging
	hash := sha256.Sum256(image)
	imageHash := hex.EncodeToString(hash[:])

	// 3. Extract and score date candidates
	candidates := c.extractDateCandidates(rawText)

	// 4. Select best date
	best := c.selectBestDate(candidates)

	// 5. Log scan
	c.Logger.Debug().
		Str("imageHash", imageHash).
		Str("rawText", rawText).
		Str("detectedDate", best.DetectedDate).
		Float64("confidence", best.Confidence).
		Msg("OCR scan completed")

	return best, nil
}

// runTesseract executes the tesseract binary on the provided image bytes
func runTesseract(ctx context.Context, img []byte, langs string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "tesseract", "stdin", "stdout", "-l", langs, "--dpi", "300")
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

	// German/EU expiry keywords (case-insensitive)
	keywordRe := regexp.MustCompile(`(?i)(Mindesthaltbarkeit|Verbrauch[_\s]?bis|Haltbar[_\s]?bis|Mindestens[_\s]?haltbar[_\s]?bis|Zu[_\s]?verbrauchen[_\s]?bis|Gültig[_\s]?bis|Best[_\s]?before|Expiry|Use[_\s]?by)`)
	hasKeyword := keywordRe.MatchString(text)

	// Date patterns (try all)
	patterns := []struct {
		regex    *regexp.Regexp
		layout   string
		dayFirst bool
	}{
		{regexp.MustCompile(`(\d{1,2})\.(\d{1,2})\.(\d{2,4})`), "02.01.2006", true}, // DD.MM.YYYY
		{regexp.MustCompile(`(\d{1,2})/(\d{1,2})/(\d{2,4})`), "02/01/2006", true},   // DD/MM/YYYY (EU)
		{regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`), "2006-01-02", false},        // ISO
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
			// Reasonable range: past 30 days to 5 years future
			if t.Before(now.AddDate(0, 0, -30)) || t.After(now.AddDate(5, 0, 0)) {
				continue
			}
			// Confidence baseline
			conf := 0.4 // baseline for plausible date
			if hasKeyword {
				conf += 0.3
				// Bonus if date appears within 20 chars of a keyword
				lowerText := strings.ToLower(text)
				kwIdx := keywordRe.FindStringIndex(lowerText)
				dateIdx := strings.Index(lowerText, strings.ToLower(raw))
				if kwIdx != nil && kwIdx[0] >= 0 && dateIdx >= 0 && abs(kwIdx[0]-dateIdx) <= 20 {
					conf += 0.1
				}
			}
			// Day-first formats are more common in EU → small boost
			if p.dayFirst {
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
	// parts[0]=day or month, parts[1]=month or day, parts[2]=year
	day, month, year := parts[0], parts[1], parts[2]
	if !dayFirst {
		day, month = month, day
	}
	// Pad day/month to 2 digits
	if len(day) == 1 {
		day = "0" + day
	}
	if len(month) == 1 {
		month = "0" + month
	}
	// Year: if 2-digit, assume 20xx (EU packaging usually > 2020)
	if len(year) == 2 {
		year = "20" + year
	}
	return fmt.Sprintf("%s-%s-%s", year, month, day) // time.Parse accepts YYYY-MM-DD
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
		DetectedDate: best.date.Format("2006-01-02"),
		Confidence:   best.confidence,
		RawText:      best.raw,
	}
}
