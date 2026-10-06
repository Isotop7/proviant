package controllers

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
)

// receiptStreamMaxContentBytes caps the total delta content accumulated from
// one scan stream. Real receipt JSON stays far below 1MB (ReceiptScanMaxTokens
// bounds the completion), so a larger accumulation means a broken or hostile
// endpoint flooding memory until the scan deadline; the stream is aborted.
const receiptStreamMaxContentBytes = 1 << 20

// receiptPrompt instructs the vision model to return strict JSON line items.
// Kept in English: receipt photos are multilingual and every mainstream VLM
// follows language-neutral extraction instructions more reliably than
// localized ones.
const receiptPrompt = "You are reading a photo of a grocery receipt. Extract every product line item. " +
	"Return ONLY a JSON object, no markdown fences, no commentary: " +
	`{"items":[{"name":string,"amount":integer,"unit":string,"price":number}]}. ` +
	"Rules: name is the product description without quantities or prices; " +
	"amount is the purchased quantity as integer (default 1); unit is a short unit like pcs, kg, g, l, ml or empty; " +
	"price is the line total in the receipt currency, omit it when unreadable. " +
	"Ignore totals, taxes, payment details and non-product lines. " +
	"If nothing on the image is a product line item, return {\"items\":[]}. " +
	"When unsure about a value, keep the closest plausible reading instead of inventing products."

// ReceiptScanController extracts line items from receipt photos via an
// OpenAI-compatible vision endpoint
type ReceiptScanController interface {
	ScanReceipt(parent context.Context, image []byte) (*apiModel.ReceiptScanResponse, error)
}

type ReceiptScanControllerImpl struct {
	Logger *zerolog.Logger
	Config configuration.ReceiptOCRConfiguration
	Client *http.Client
}

// NewReceiptScanController creates a new receipt scan controller instance
func NewReceiptScanController(logger *zerolog.Logger, config *configuration.ReceiptOCRConfiguration) *ReceiptScanControllerImpl {
	// One shared client for all scans so connections are pooled and reused.
	// Tests can override via the Client field.
	scanTimeout := config.Timeout
	if scanTimeout <= 0 {
		scanTimeout = util.ReceiptScanDefaultTimeout
	}
	return &ReceiptScanControllerImpl{
		Logger: logger,
		Config: *config,
		Client: &http.Client{Timeout: time.Duration(scanTimeout) * time.Second},
	}
}

type receiptChatRequest struct {
	Model     string               `json:"model"`
	Messages  []receiptChatMessage `json:"messages"`
	MaxTokens int                  `json:"max_tokens"`
	Stream    bool                 `json:"stream"`
}

type receiptChatMessage struct {
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
}

type receiptImageURLPart struct {
	Type     string          `json:"type"`
	ImageURL receiptImageURL `json:"image_url"`
}

type receiptImageURL struct {
	URL string `json:"url"`
}

type receiptTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type receiptChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *receiptEndpointError `json:"error"`
}

// receiptStreamChunk is one SSE data payload of a streamed chat completion.
// Some OpenAI-compatible servers report errors inside the stream with the
// same shape, so the error field is shared.
type receiptStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *receiptEndpointError `json:"error"`
}

// receiptEndpointError accepts both OpenAI-style error objects
// ({"error":{"message":"…"}}) and Ollama-style plain strings
// ({"error":"…"}). A pointer-only-object type would make the whole chunk
// unmarshal fail on the string form, silently swallowing endpoint errors.
type receiptEndpointError struct {
	Message string
}

func (e *receiptEndpointError) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return err
		}
		e.Message = text
		return nil
	}
	var obj struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return err
	}
	e.Message = obj.Message
	return nil
}

// receiptChatCompletionsURL normalizes the configured endpoint to a full
// chat-completions URL. Users point the config at a base URL ("point at
// Ollama/vLLM/LM Studio"), but those servers expose the OpenAI dialect under
// /v1/chat/completions and POSTing to the bare base returns 405. Accepted
// forms: a full chat-completions URL (used as-is), a URL ending in /v1
// (appended with /chat/completions), or a bare base URL (appended with
// /v1/chat/completions). Empty yields the public OpenAI endpoint. A query
// string (e.g. Azure's "?api-version=...") is preserved: suffix checks and
// path appending consider only the part before "?", and the query is kept
// after any appended path.
func receiptChatCompletionsURL(raw string) string {
	endpoint := strings.TrimRight(strings.TrimSpace(raw), "/")
	if endpoint == "" {
		return "https://api.openai.com/v1/chat/completions"
	}
	base, query := endpoint, ""
	if idx := strings.Index(endpoint, "?"); idx != -1 {
		base, query = endpoint[:idx], endpoint[idx:]
	}
	switch {
	case strings.HasSuffix(base, "/chat/completions"):
		return base + query
	case strings.HasSuffix(base, "/v1"):
		return base + "/chat/completions" + query
	default:
		return base + "/v1/chat/completions" + query
	}
}

// receiptRedactQuery strips the query string from a URL so secrets passed as
// query parameters (e.g. Azure's api-version or Gemini's key) never reach the
// server logs. Scheme, host and path are kept; invalid URLs are returned as-is.
func receiptRedactQuery(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// receiptImageMIMETypes are the image formats accepted for receipt scans
var receiptImageMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// ReceiptImageMIME sniffs the actual image format of the uploaded bytes. It
// returns the detected MIME type when the payload is one of the accepted
// receipt image formats, and "" otherwise — the declared Content-Type header
// of a multipart upload is client-controlled and never trusted.
func ReceiptImageMIME(data []byte) string {
	mimeType := http.DetectContentType(data)
	if receiptImageMIMETypes[mimeType] {
		return mimeType
	}
	return ""
}

// ScanReceipt sends the receipt photo to the configured vision endpoint and
// returns validated line items. An unreadable receipt is a successful scan
// with zero items; only transport, auth or timeout problems are errors.
// The scan runs as a child of the passed context, so a caller-side timeout
// also cancels the outbound request. Its own config timeout is layered on
// top so direct callers without a deadline are still bounded.
func (c *ReceiptScanControllerImpl) ScanReceipt(parent context.Context, image []byte) (*apiModel.ReceiptScanResponse, error) {
	timeout := c.Config.Timeout
	if timeout <= 0 {
		timeout = util.ReceiptScanDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeout)*time.Second)
	defer cancel()

	endpoint := receiptChatCompletionsURL(c.Config.Endpoint)
	startedAt := time.Now()
	// The data URL prefix must match the real bytes, not an assumed format:
	// endpoints reject PNG/WebP payloads labelled as JPEG. A non-image payload
	// is a caller error — guessing a MIME type would send arbitrary bytes to
	// the vision endpoint as if they were a photo.
	imageMIME := ReceiptImageMIME(image)
	if imageMIME == "" {
		return nil, proviantErrors.ErrInvalidImageType
	}
	c.Logger.Debug().
		Str("endpoint", receiptRedactQuery(endpoint)).
		Str("model", c.Config.Model).
		Int("imageBytes", len(image)).
		Int("timeoutSec", timeout).
		Msgf("Receipt scan started")

	reqBody := receiptChatRequest{
		Model: c.Config.Model,
		Messages: []receiptChatMessage{{
			Role: "user",
			Content: []interface{}{
				receiptImageURLPart{
					Type:     "image_url",
					ImageURL: receiptImageURL{URL: "data:" + imageMIME + ";base64," + base64.StdEncoding.EncodeToString(image)},
				},
				receiptTextPart{Type: "text", Text: receiptPrompt},
			},
		}},
		MaxTokens: util.ReceiptScanMaxTokens,
		// Streamed so the model's raw output can be followed live in the
		// server console while a scan runs.
		Stream: true,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("receipt scan request encoding failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("receipt scan request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.Config.APIKey)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, &proviantErrors.ReceiptEndpointError{Err: fmt.Errorf("receipt scan endpoint unreachable: %w", err)}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		c.Logger.Warn().
			Int("status", resp.StatusCode).
			Str("duration", time.Since(startedAt).String()).
			Str("body", truncateForLog(errBody, 500)).
			Msg("Receipt scan endpoint returned non-200")
		return nil, &proviantErrors.ReceiptEndpointError{Err: fmt.Errorf("receipt scan endpoint error (HTTP %d)", resp.StatusCode)}
	}

	// Servers without streaming support ignore stream:true and answer with a
	// regular JSON completion; only an SSE content type is consumed as a
	// stream. Either way the accumulated content is identical afterwards.
	// The SSE body must be streamed from resp.Body directly — reading it into
	// a buffer first would drain the reader before the scanner sees it.
	var rawContent string
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		content, streamErr := c.streamChatResponse(resp.Body, startedAt)
		if streamErr != nil {
			return nil, streamErr
		}
		rawContent = content
	} else {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, fmt.Errorf("receipt scan response read failed: %w", err)
		}
		content, plainErr := c.readPlainChatResponse(body, startedAt)
		if plainErr != nil {
			return nil, plainErr
		}
		rawContent = content
	}

	items, parseErr := ParseReceiptItems(rawContent)
	if parseErr != nil {
		// Log length + hash only: the raw output carries extracted receipt
		// data (product names, prices) that must not land in server logs.
		c.Logger.Warn().
			Str("duration", time.Since(startedAt).String()).
			Int("rawLength", len(rawContent)).
			Str("rawHash", fmt.Sprintf("%x", sha256.Sum256([]byte(rawContent)))).
			Msg("Receipt scan model output could not be parsed")
		return nil, parseErr
	}

	c.Logger.Debug().
		Str("duration", time.Since(startedAt).String()).
		Int("items", len(items)).
		Msg("Receipt scan completed")
	return &apiModel.ReceiptScanResponse{Items: items}, nil
}

// streamChatResponse consumes an OpenAI-style SSE stream and returns the
// concatenated delta content.
func (c *ReceiptScanControllerImpl) streamChatResponse(body io.Reader, startedAt time.Time) (string, error) {
	var content strings.Builder
	contentBytes := 0
	chunks := 0
	scanner := bufio.NewScanner(body)
	// 8MB line cap: SSE data lines are small JSON deltas, but some endpoints
	// embed whole base64 images or large usage blocks in one event; a cap too
	// tight would abort the entire scan with bufio.ErrTooLong (handled below).
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		// Single-line data assumption: the SSE spec allows one event's data
		// split across consecutive `data:` lines (joined with \n), but every
		// mainstream OpenAI-compatible server (OpenAI, Ollama, vLLM, LM
		// Studio) emits one JSON delta per line. If an endpoint ever splits,
		// the fragment fails Unmarshal, is logged above, and the scan degrades
		// to partial content — not a crash. Do not "fix" by accumulating
		// lines without a blank-line event boundary check.
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk receiptStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// Log length + hash only: stream deltas carry extracted receipt
			// data (product names, prices) that must not land in server logs.
			c.Logger.Warn().
				Int("lineLength", len(data)).
				Str("lineHash", fmt.Sprintf("%x", sha256.Sum256([]byte(data)))).
				Msg("Receipt scan stream contained an unparsable data line")
			continue
		}
		// Some servers report Ollama-native errors as {"error":"<string>"}
		// inside the stream; surface it instead of silently yielding nothing.
		if chunk.Error != nil && chunk.Error.Message != "" {
			c.Logger.Warn().Str("endpointError", chunk.Error.Message).Msg("Receipt scan endpoint reported an error mid-stream")
			return "", &proviantErrors.ReceiptEndpointError{Err: fmt.Errorf("receipt scan endpoint error: %s", chunk.Error.Message)}
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			// The 8MB scanner cap only bounds a single line; without a total
			// cap, endless small deltas balloon memory until the deadline.
			// Check BEFORE appending so accumulation is hard-bounded at the
			// cap and can never overshoot by one delta (up to 8MB).
			if contentBytes+len(choice.Delta.Content) > receiptStreamMaxContentBytes {
				c.Logger.Warn().
					Int("bytes", contentBytes).
					Msg("Receipt scan stream exceeded the total content cap")
				return "", &proviantErrors.ReceiptEndpointError{Err: fmt.Errorf("receipt scan stream exceeded the %d byte content cap", receiptStreamMaxContentBytes)}
			}
			content.WriteString(choice.Delta.Content)
			contentBytes += len(choice.Delta.Content)
			chunks++
		}
	}
	if err := scanner.Err(); err != nil {
		// A stream that merely ends without a trailing newline after the last
		// event is NOT an error: bufio.Scanner returns the final partial line
		// and a nil error, so it never reaches this branch. An error here —
		// including io.ErrUnexpectedEOF from a truncated chunked body — is a
		// genuine mid-stream disconnect, and the accumulated output may be cut
		// mid-JSON. Failing the scan beats silently yielding a partial item
		// list the user cannot distinguish from a complete one.
		if errors.Is(err, bufio.ErrTooLong) {
			c.Logger.Warn().Msg("Receipt scan stream contained a data line exceeding the 8MB scanner cap")
			return "", &proviantErrors.ReceiptEndpointError{Err: fmt.Errorf("receipt scan stream line too long")}
		}
		return "", fmt.Errorf("receipt scan stream read failed: %w", err)
	}
	c.Logger.Debug().
		Str("duration", time.Since(startedAt).String()).
		Int("chunks", chunks).
		Int("bytes", contentBytes).
		Msg("Receipt scan stream finished")
	return content.String(), nil
}

// readPlainChatResponse parses a non-streaming chat completion body.
func (c *ReceiptScanControllerImpl) readPlainChatResponse(body []byte, startedAt time.Time) (string, error) {
	var chatResp receiptChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		// Log length + hash only: the raw body carries extracted receipt
		// data (product names, prices) that must not land in server logs.
		c.Logger.Warn().
			Str("duration", time.Since(startedAt).String()).
			Int("bodyLength", len(body)).
			Str("bodyHash", fmt.Sprintf("%x", sha256.Sum256(body))).
			Msg("Receipt scan response is not valid JSON")
		return "", fmt.Errorf("receipt scan response parse failed: %w", err)
	}
	if chatResp.Error != nil {
		c.Logger.Warn().Str("endpointError", chatResp.Error.Message).Msg("Receipt scan endpoint reported an error")
		return "", &proviantErrors.ReceiptEndpointError{Err: fmt.Errorf("receipt scan endpoint error: %s", chatResp.Error.Message)}
	}
	if len(chatResp.Choices) == 0 {
		c.Logger.Warn().Str("duration", time.Since(startedAt).String()).Msg("Receipt scan endpoint returned no choices")
		return "", &proviantErrors.ReceiptEndpointError{Err: fmt.Errorf("receipt scan endpoint returned no choices")}
	}
	content := chatResp.Choices[0].Message.Content
	return content, nil
}

// truncateForLog bounds a payload for log output so a large or broken model
// response cannot flood the server log. A marker records when truncation
// happened instead of silently losing data.
func truncateForLog(data []byte, max int) string {
	if len(data) <= max {
		return string(data)
	}
	return truncateUTF8(string(data), max) + fmt.Sprintf("… (truncated, %d bytes total)", len(data))
}

// truncateUTF8 shortens a string to at most max bytes without splitting a
// multi-byte rune, so the result stays valid UTF-8 for DB storage and logs.
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	used := 0
	for i, r := range s {
		size := utf8.RuneLen(r)
		if i+size > max {
			return s[:i]
		}
		used = i + size
	}
	return s[:used]
}

// tolerantReceiptItem mirrors ReceiptItem but accepts non-integer amounts and
// omittable prices, as small vision models routinely produce them.
type tolerantReceiptItem struct {
	Name   string   `json:"name"`
	Amount float64  `json:"amount"`
	Unit   string   `json:"unit"`
	Price  *float64 `json:"price"`
}

type tolerantReceiptPayload struct {
	Items []tolerantReceiptItem `json:"items"`
}

// ParseReceiptItems extracts the JSON payload from the model output and
// validates/clamps every item. Invalid items are dropped, not fatal: a
// half-recognized receipt is still worth showing in the review UI. Vision
// models often emit near-JSON (missing commas between items, non-integer
// amounts, output truncated at the token cap), so a strict parse is retried
// over a repaired payload, and as a last resort complete items are salvaged
// from an unparseable one before giving up. Commentary around the payload may
// contain stray brace pairs, so every top-level object is tried in order and
// the first one yielding items wins — an earlier brace pair must not mask the
// real payload behind a silent empty success.
func ParseReceiptItems(content string) ([]apiModel.ReceiptItem, error) {
	payloads := extractReceiptJSONPayloads(content)
	if len(payloads) == 0 {
		return nil, fmt.Errorf("receipt scan model output contains no JSON object")
	}

	var decodeErr error
	var salvaged []apiModel.ReceiptItem
	decodedAny := false
	for _, payload := range payloads {
		parsed, err := decodeReceiptItems(payload)
		if err != nil {
			if decodeErr == nil {
				decodeErr = err
			}
			if len(salvaged) == 0 {
				salvaged = salvageReceiptItems(payload)
			}
			continue
		}
		decodedAny = true
		items := buildReceiptItems(parsed)
		if len(items) == 0 {
			continue
		}
		return items, nil
	}
	if len(salvaged) > 0 {
		return salvaged, nil
	}
	if !decodedAny && decodeErr != nil {
		return nil, fmt.Errorf("receipt scan model output is not valid JSON: %w", decodeErr)
	}
	// Every object decoded but none contained items: a legitimately empty
	// receipt stays an empty success.
	return buildReceiptItems(nil), nil
}

// decodeReceiptItems parses the payload, falling back to a repaired variant.
func decodeReceiptItems(payload string) ([]tolerantReceiptItem, error) {
	var parsed tolerantReceiptPayload
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		if err2 := json.Unmarshal([]byte(repairReceiptJSON(payload)), &parsed); err2 != nil {
			return nil, err
		}
	}
	return parsed.Items, nil
}

// buildReceiptItems trims/clamps parsed model output into API items.
func buildReceiptItems(parsed []tolerantReceiptItem) []apiModel.ReceiptItem {
	items := make([]apiModel.ReceiptItem, 0, len(parsed))
	for _, item := range parsed {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if len(name) > util.ReceiptItemMaxNameLength {
			name = truncateUTF8(name, util.ReceiptItemMaxNameLength)
		}
		amount := int(math.Round(item.Amount))
		if amount < 1 {
			amount = 1
		}
		if amount > util.ReceiptItemMaxAmount {
			amount = util.ReceiptItemMaxAmount
		}
		unit := strings.TrimSpace(item.Unit)
		if len(unit) > util.ReceiptItemMaxUnitLength {
			unit = truncateUTF8(unit, util.ReceiptItemMaxUnitLength)
		}
		cleaned := apiModel.ReceiptItem{Name: name, Amount: amount, Unit: unit}
		if item.Price != nil && *item.Price > 0 && *item.Price <= util.ReceiptItemMaxPrice {
			price := *item.Price
			cleaned.Price = &price
		}
		items = append(items, cleaned)
	}
	return items
}

// salvageReceiptItems recovers individually parseable items from a malformed
// or token-truncated payload: the output is split on item starts, each chunk
// is closed, repaired and parsed on its own, and broken chunks are skipped.
func salvageReceiptItems(payload string) []apiModel.ReceiptItem {
	const itemStart = `{"name"`
	idx := strings.Index(payload, itemStart)
	if idx < 0 {
		return nil
	}
	parts := strings.Split(payload[idx+len(itemStart):], itemStart)
	items := make([]apiModel.ReceiptItem, 0, len(parts))
	for _, part := range parts {
		// Drop everything after the array's closing bracket: a trailing
		// truncated item or a "total" block would poison the chunk. The
		// bracket search must skip `]` inside string literals — a product
		// name like "Milk [1L]" would otherwise truncate the object mid-string.
		if bracket := indexJSONBracketOutsideString(part); bracket >= 0 {
			part = part[:bracket]
		}
		var candidate string
		if end := strings.LastIndex(part, "}"); end >= 0 {
			// Complete object: cut the trailing separator (", " …) after it.
			candidate = itemStart + part[:end+1]
		} else {
			// Truncated object: drop dangling separators and close it. The
			// closing quote of the last value must survive the trim.
			candidate = itemStart + strings.TrimRight(part, " \t\r\n,:") + "}"
		}
		parsed, err := decodeReceiptItems(`{"items":[` + repairReceiptJSON(candidate) + `]}`)
		if err != nil || len(parsed) == 0 {
			continue
		}
		items = append(items, buildReceiptItems(parsed)...)
	}
	return items
}

// indexJSONBracketOutsideString returns the index of the first `]` byte that
// is not inside a JSON string literal, or -1. String-aware bracket counting
// mirrors extractReceiptJSONPayloads: a `]` inside a quoted value is data, not
// structure.
func indexJSONBracketOutsideString(s string) int {
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case ']':
			return i
		}
	}
	return -1
}

// extractReceiptJSONPayloads returns every complete top-level JSON object in
// the model output, found by bracket counting that respects string literals.
// Models wrap the object in markdown fences and commentary — including stray
// brace pairs like "found {3} items:" — so every candidate is returned and the
// caller picks the first one that actually yields items. An unterminated final
// object (token-cap truncation) is appended as-is so the repair/salvage path
// can still work with it.
func extractReceiptJSONPayloads(content string) []string {
	start := strings.Index(content, "{")
	if start < 0 {
		return nil
	}
	var payloads []string
	objStart := -1
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(content); i++ {
		c := content[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				objStart = i
			}
			depth++
		case '}':
			// A stray closing brace outside strings (e.g. commentary like
			// "found } items") must not push depth negative and desync the
			// scanner — clamp to 0 so the next real '{' still captures.
			if depth > 0 {
				depth--
				if depth == 0 {
					payloads = append(payloads, content[objStart:i+1])
				}
			}
		}
	}
	if depth > 0 && objStart >= 0 {
		payloads = append(payloads, content[objStart:])
	}
	return payloads
}

// repairReceiptJSON fixes the malformations small vision models actually
// produce: a missing comma between consecutive array elements (`}\n{`), a
// missing closing brace + comma when the previous element's object is still
// open (`"amount":1\n{`), and a trailing comma before a closing bracket
// (`},]`). It runs only after a strict parse has failed, so it never touches
// well-formed output. The scan is string-aware: a pattern occurring inside a
// JSON string literal (e.g. a product name ending in an escaped quote right
// before a brace) is left alone instead of being corrupted into a separator.
func repairReceiptJSON(payload string) string {
	repaired := make([]byte, 0, len(payload)+16)
	var stack []byte
	inString := false
	escaped := false
	lastMeaningful := byte(0)
	for i := 0; i < len(payload); i++ {
		c := payload[i]
		if inString {
			repaired = append(repaired, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
				lastMeaningful = '"'
			}
			continue
		}
		if c == '{' && !isReceiptJSONStructuralPrefix(lastMeaningful) {
			// An object starts where a separator (and possibly a closing
			// brace) belongs. If the previous array element's object is
			// still open, its `{` sits on the stack directly above the
			// array and must be closed before the new object; an element
			// that was already closed only misses the comma.
			if len(stack) > 1 && stack[len(stack)-1] == '{' && stack[len(stack)-2] == '[' {
				repaired = append(repaired, "},"...)
				stack = stack[:len(stack)-1]
			} else {
				repaired = append(repaired, ',')
			}
		}
		switch c {
		case '"':
			inString = true
			lastMeaningful = '"'
		case '{', '[':
			stack = append(stack, c)
			lastMeaningful = c
		case '}', ']':
			// A comma directly before a closing bracket is always
			// invalid JSON: drop it (with surrounding whitespace)
			// instead of copying it verbatim.
			if lastMeaningful == ',' {
				repaired = bytes.TrimRight(repaired, " \t\r\n,")
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			lastMeaningful = c
		default:
			if !isReceiptJSONSpace(c) {
				lastMeaningful = c
			}
		}
		repaired = append(repaired, c)
	}
	return string(repaired)
}

// isReceiptJSONStructuralPrefix reports whether a `{` may legally follow the
// given last byte outside a string: array/object start, a separator or a
// key-value colon.
func isReceiptJSONStructuralPrefix(last byte) bool {
	switch last {
	case 0, '[', '{', ',', ':':
		return true
	}
	return false
}

func isReceiptJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
