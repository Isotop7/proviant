package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"

	"github.com/rs/zerolog"
)

func newReceiptScanController(t *testing.T) *ReceiptScanControllerImpl {
	t.Helper()
	logger := zerolog.Nop()
	return NewReceiptScanController(&logger, &configuration.ReceiptOCRConfiguration{
		Enabled: true,
		Model:   "test-model",
		Timeout: 5,
	})
}

func TestReceiptImageMIME(t *testing.T) {
	pngBytes := &bytes.Buffer{}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(pngBytes, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "png is detected", data: pngBytes.Bytes(), want: "image/png"},
		{name: "jpeg magic is detected", data: []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, want: "image/jpeg"},
		{name: "webp magic is detected", data: []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), want: "image/webp"},
		{name: "pdf is rejected", data: []byte("%PDF-1.4"), want: ""},
		{name: "text is rejected", data: []byte("hello world"), want: ""},
		{name: "empty is rejected", data: nil, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReceiptImageMIME(tt.data); got != tt.want {
				t.Errorf("ReceiptImageMIME(%q) = %q, want %q", tt.data, got, tt.want)
			}
		})
	}
}

func TestReceiptScanControllerRejectsNonImage(t *testing.T) {
	ctrl := newReceiptScanController(t)
	resp, err := ctrl.ScanReceipt(t.Context(), []byte("%PDF-1.4 not an image"))
	if err == nil {
		t.Fatalf("expected error for non-image payload, got resp=%+v", resp)
	}
}

func TestReceiptChatCompletionsURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty yields OpenAI default", raw: "", want: "https://api.openai.com/v1/chat/completions"},
		{name: "Ollama base URL gets full path", raw: "http://127.0.0.1:11434", want: "http://127.0.0.1:11434/v1/chat/completions"},
		{name: "trailing slash is trimmed first", raw: "http://127.0.0.1:11434/", want: "http://127.0.0.1:11434/v1/chat/completions"},
		{name: "URL ending in /v1 gets only chat completions", raw: "http://localhost:8000/v1", want: "http://localhost:8000/v1/chat/completions"},
		{name: "full chat completions URL is kept", raw: "http://localhost:1234/v1/chat/completions", want: "http://localhost:1234/v1/chat/completions"},
		{name: "whitespace is trimmed", raw: "  http://127.0.0.1:11434  ", want: "http://127.0.0.1:11434/v1/chat/completions"},
		{name: "whitespace-only yields OpenAI default", raw: "   ", want: "https://api.openai.com/v1/chat/completions"},
		{name: "Azure-style query with full path is kept", raw: "https://x.openai.azure.com/openai/deployments/d/chat/completions?api-version=2024-02-01", want: "https://x.openai.azure.com/openai/deployments/d/chat/completions?api-version=2024-02-01"},
		{name: "query with missing path gets path before query", raw: "https://x.openai.azure.com/openai/deployments/d?api-version=2024-02-01", want: "https://x.openai.azure.com/openai/deployments/d/v1/chat/completions?api-version=2024-02-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := receiptChatCompletionsURL(tt.raw); got != tt.want {
				t.Errorf("receiptChatCompletionsURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// TestRepairReceiptJSON runs the repairer over the malformations small vision
// models actually produce and asserts every repaired payload parses. A
// payload that fails to parse here would fall through to the salvage path,
// losing items that repair was supposed to save.
func TestRepairReceiptJSON(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		wantNames []string
	}{
		{
			name:      "valid JSON is passed through unchanged",
			payload:   `{"items":[{"name":"Milk","amount":1,"unit":"l","price":1.29}]}`,
			wantNames: []string{"Milk"},
		},
		{
			name:      "trailing comma before the closing bracket",
			payload:   `{"items":[{"name":"Milk"},{"name":"Bread"},]}`,
			wantNames: []string{"Milk", "Bread"},
		},
		{
			name:      "missing comma between closed items",
			payload:   "{\"items\":[{\"name\":\"Milk\"}\n{\"name\":\"Bread\"}]}",
			wantNames: []string{"Milk", "Bread"},
		},
		{
			name:      "previous item object left open after a string value",
			payload:   "{\"items\":[{\"name\":\"Milk\",\"unit\":\"l\"\n{\"name\":\"Bread\",\"unit\":\"pcs\"}]}",
			wantNames: []string{"Milk", "Bread"},
		},
		{
			name:      "previous item object left open after a number value",
			payload:   "{\"items\":[{\"name\":\"Milk\",\"amount\":2\n{\"name\":\"Bread\"}]}",
			wantNames: []string{"Milk", "Bread"},
		},
		{
			// The `}{` sequence inside a product name is data, not a
			// separator, and must survive the repair untouched.
			name:      "brace pattern inside a string literal is left alone",
			payload:   `{"items":[{"name":"Milk }{ fake","amount":1}]}`,
			wantNames: []string{"Milk }{ fake"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repaired := repairReceiptJSON(tt.payload)
			var parsed tolerantReceiptPayload
			if err := json.Unmarshal([]byte(repaired), &parsed); err != nil {
				t.Fatalf("repaired payload does not parse: %v\npayload:   %s\nrepaired:  %s", err, tt.payload, repaired)
			}
			if len(parsed.Items) != len(tt.wantNames) {
				t.Fatalf("repaired payload has %d items, want %d\nrepaired: %s", len(parsed.Items), len(tt.wantNames), repaired)
			}
			for i, want := range tt.wantNames {
				if parsed.Items[i].Name != want {
					t.Errorf("item %d name = %q, want %q", i, parsed.Items[i].Name, want)
				}
			}
		})
	}
}

func TestRepairReceiptJSON_LeavesValidJSONByteIdentical(t *testing.T) {
	payload := `{"items":[{"name":"Milk","amount":1},{"name":"Bread","amount":2,"price":2.5}]}`
	if got := repairReceiptJSON(payload); got != payload {
		t.Errorf("repairReceiptJSON() changed well-formed output:\n got:  %s\nwant: %s", got, payload)
	}
}

// TestParseReceiptItemsMultipleObjects covers commentary containing stray
// brace pairs before or after the real payload: the first extracted object
// must not mask a later object holding the actual items.
func TestParseReceiptItemsMultipleObjects(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantNames []string
	}{
		{
			name:      "commentary brace pair before the payload",
			content:   `I found {3} items on the receipt: {"items":[{"name":"Milk","amount":1,"unit":"l"}]}`,
			wantNames: []string{"Milk"},
		},
		{
			name:      "commentary brace pair after the payload",
			content:   `{"items":[{"name":"Bread","amount":2}]} that is {2} items in total`,
			wantNames: []string{"Bread"},
		},
		{
			name:      "empty payload object before the real payload",
			content:   `{"items":[]} retry: {"items":[{"name":"Butter","amount":1}]}`,
			wantNames: []string{"Butter"},
		},
		{
			// Two stray closing braces before the payload previously pushed
			// the depth negative, desyncing the scanner so the real object
			// was silently dropped and the scan "succeeded" with zero items.
			name:      "stray closing braces before the payload",
			content:   `} } {"items":[{"name":"Milk","amount":1,"unit":"l"}]}`,
			wantNames: []string{"Milk"},
		},
		{
			name:      "single valid object is returned unchanged",
			content:   `{"items":[{"name":"Milk","amount":1},{"name":"Bread","amount":2}]}`,
			wantNames: []string{"Milk", "Bread"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := ParseReceiptItems(tt.content)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(items) != len(tt.wantNames) {
				t.Fatalf("items = %d (%+v), want %d", len(items), items, len(tt.wantNames))
			}
			for i, want := range tt.wantNames {
				if items[i].Name != want {
					t.Errorf("item %d name = %q, want %q", i, items[i].Name, want)
				}
			}
		})
	}

	t.Run("zero items anywhere stays an empty success", func(t *testing.T) {
		items, err := ParseReceiptItems(`{"items":[]} nothing found, checked {1} object`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("items = %+v, want empty", items)
		}
	})

	t.Run("unparseable objects without items still error", func(t *testing.T) {
		if _, err := ParseReceiptItems("{not json} {also broken}"); err == nil {
			t.Errorf("expected error for output without any valid JSON object")
		}
	})
}

func TestStreamChatResponseAbortsOnContentCap(t *testing.T) {
	logger := zerolog.Nop()
	ctrl := ReceiptScanControllerImpl{Logger: &logger}

	var stream bytes.Buffer
	// 64-byte deltas: individually far below the 8MB line cap, but many of
	// them exceed the 1MB total content cap.
	delta := `{"choices":[{"delta":{"content":"` + strings.Repeat("x", 64) + `"}}]}`
	for stream.Len() < 2*receiptStreamMaxContentBytes {
		stream.WriteString("data: " + delta + "\n\n")
	}

	_, err := ctrl.streamChatResponse(&stream, time.Now())
	if err == nil {
		t.Fatalf("expected error when stream exceeds the content cap")
	}
	if !strings.Contains(err.Error(), "content cap") {
		t.Errorf("error = %v, want content-cap failure", err)
	}
	// The flood is caused by the endpoint, so it must classify as an
	// endpoint error (502), not an internal processing error (500).
	var endpointErr *proviantErrors.ReceiptEndpointError
	if !errors.As(err, &endpointErr) {
		t.Errorf("error = %T (%v), want *proviantErrors.ReceiptEndpointError", err, err)
	}
}
