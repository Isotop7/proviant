package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
)

func testPNGBytes(t *testing.T) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func newExtraReceiptController(endpoint string) *ReceiptScanControllerImpl {
	logger := zerolog.Nop()
	return NewReceiptScanController(&logger, &configuration.ReceiptOCRConfiguration{
		Enabled:  true,
		Model:    "test-model",
		Endpoint: endpoint,
		Timeout:  5,
	})
}

func TestNewReceiptScanControllerDefaultTimeout(t *testing.T) {
	logger := zerolog.Nop()
	ctrl := NewReceiptScanController(&logger, &configuration.ReceiptOCRConfiguration{})
	want := time.Duration(util.ReceiptScanDefaultTimeout) * time.Second
	if ctrl.Client.Timeout != want {
		t.Errorf("Client.Timeout = %v, want %v (default)", ctrl.Client.Timeout, want)
	}
}

func TestReceiptEndpointErrorUnmarshalJSON(t *testing.T) {
	t.Run("Ollama-style string error", func(t *testing.T) {
		var e receiptEndpointError
		if err := json.Unmarshal([]byte(`"model not found"`), &e); err != nil {
			t.Fatalf("UnmarshalJSON() = %v", err)
		}
		if e.Message != "model not found" {
			t.Errorf("Message = %q, want %q", e.Message, "model not found")
		}
	})

	t.Run("OpenAI-style object error", func(t *testing.T) {
		var e receiptEndpointError
		if err := json.Unmarshal([]byte(`{"message":"rate limited"}`), &e); err != nil {
			t.Fatalf("UnmarshalJSON() = %v", err)
		}
		if e.Message != "rate limited" {
			t.Errorf("Message = %q, want %q", e.Message, "rate limited")
		}
	})

	t.Run("invalid JSON errors", func(t *testing.T) {
		var e receiptEndpointError
		if err := json.Unmarshal([]byte(`123`), &e); err == nil {
			t.Error("UnmarshalJSON(123) = nil, want error")
		}
	})
}

func TestReceiptRedactQuery(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "query stripped", raw: "https://x.example.com/path?key=secret&v=2", want: "https://x.example.com/path"},
		{name: "fragment stripped", raw: "https://x.example.com/path#section", want: "https://x.example.com/path"},
		{name: "no query unchanged", raw: "https://x.example.com/path", want: "https://x.example.com/path"},
		{name: "invalid URL returned as-is", raw: "://bad url", want: "://bad url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := receiptRedactQuery(tt.raw); got != tt.want {
				t.Errorf("receiptRedactQuery(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestScanReceiptHTTPPaths(t *testing.T) {
	imageBytes := testPNGBytes(t)

	t.Run("endpoint unreachable", func(t *testing.T) {
		ctrl := newExtraReceiptController("http://127.0.0.1:1")
		_, err := ctrl.ScanReceipt(context.Background(), imageBytes)
		var endpointErr *proviantErrors.ReceiptEndpointError
		if !errors.As(err, &endpointErr) {
			t.Errorf("err = %v (%T), want *ReceiptEndpointError", err, err)
		}
	})

	t.Run("non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"bad key"}`))
		}))
		defer server.Close()
		ctrl := newExtraReceiptController(server.URL)
		_, err := ctrl.ScanReceipt(context.Background(), imageBytes)
		var endpointErr *proviantErrors.ReceiptEndpointError
		if !errors.As(err, &endpointErr) {
			t.Fatalf("err = %v (%T), want *ReceiptEndpointError", err, err)
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("err = %v, want the HTTP status in the message", err)
		}
	})

	t.Run("plain JSON completion success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer secret" {
				t.Error("Authorization header missing")
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices":[{"message":{"content":"{\"items\":[{\"name\":\"Milk\",\"amount\":2,\"unit\":\"l\",\"price\":1.5}]}"}}]}`))
		}))
		defer server.Close()
		ctrl := newExtraReceiptController(server.URL)
		ctrl.Config.APIKey = "secret"
		resp, err := ctrl.ScanReceipt(context.Background(), imageBytes)
		if err != nil {
			t.Fatalf("ScanReceipt() = %v", err)
		}
		if len(resp.Items) != 1 || resp.Items[0].Name != "Milk" || resp.Items[0].Amount != 2 {
			t.Errorf("Items = %+v, want one Milk x2", resp.Items)
		}
		if resp.Items[0].Price == nil || *resp.Items[0].Price != 1.5 {
			t.Errorf("Price = %v, want 1.5", resp.Items[0].Price)
		}
	})

	t.Run("streamed SSE completion success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"items\\\":[{\\\"name\\\":\\\"Bread\\\"}\"}}]}\n\n"))
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\",\\\"amount\\\":1}]}}\"}}\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
		}))
		defer server.Close()
		ctrl := newExtraReceiptController(server.URL)
		resp, err := ctrl.ScanReceipt(context.Background(), imageBytes)
		if err != nil {
			t.Fatalf("ScanReceipt() = %v", err)
		}
		if len(resp.Items) != 1 || resp.Items[0].Name != "Bread" || resp.Items[0].Amount != 1 {
			t.Errorf("Items = %+v, want one Bread x1 assembled from stream deltas", resp.Items)
		}
	})

	t.Run("endpoint error object in plain body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"error":{"message":"model overloaded"}}`))
		}))
		defer server.Close()
		ctrl := newExtraReceiptController(server.URL)
		_, err := ctrl.ScanReceipt(context.Background(), imageBytes)
		var endpointErr *proviantErrors.ReceiptEndpointError
		if !errors.As(err, &endpointErr) {
			t.Fatalf("err = %v (%T), want *ReceiptEndpointError", err, err)
		}
		if !strings.Contains(err.Error(), "model overloaded") {
			t.Errorf("err = %v, want the endpoint message", err)
		}
	})

	t.Run("no choices in plain body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices":[]}`))
		}))
		defer server.Close()
		ctrl := newExtraReceiptController(server.URL)
		_, err := ctrl.ScanReceipt(context.Background(), imageBytes)
		var endpointErr *proviantErrors.ReceiptEndpointError
		if !errors.As(err, &endpointErr) {
			t.Fatalf("err = %v (%T), want *ReceiptEndpointError", err, err)
		}
	})

	t.Run("invalid JSON plain body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`not json at all`))
		}))
		defer server.Close()
		ctrl := newExtraReceiptController(server.URL)
		if _, err := ctrl.ScanReceipt(context.Background(), imageBytes); err == nil {
			t.Error("ScanReceipt() = nil, want parse error")
		}
	})

	t.Run("unparseable model output", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"choices":[{"message":{"content":"I cannot read this receipt"}}]}`))
		}))
		defer server.Close()
		ctrl := newExtraReceiptController(server.URL)
		if _, err := ctrl.ScanReceipt(context.Background(), imageBytes); err == nil {
			t.Error("ScanReceipt() = nil, want error for output without JSON")
		}
	})
}

func TestReadPlainChatResponse(t *testing.T) {
	logger := zerolog.Nop()
	ctrl := ReceiptScanControllerImpl{Logger: &logger}

	t.Run("invalid JSON", func(t *testing.T) {
		if _, err := ctrl.readPlainChatResponse([]byte("{broken"), time.Now()); err == nil {
			t.Error("readPlainChatResponse() = nil, want parse error")
		}
	})

	t.Run("endpoint error object", func(t *testing.T) {
		_, err := ctrl.readPlainChatResponse([]byte(`{"error":"quota exceeded"}`), time.Now())
		var endpointErr *proviantErrors.ReceiptEndpointError
		if !errors.As(err, &endpointErr) {
			t.Errorf("err = %v (%T), want *ReceiptEndpointError", err, err)
		}
	})

	t.Run("no choices", func(t *testing.T) {
		_, err := ctrl.readPlainChatResponse([]byte(`{"choices":[]}`), time.Now())
		var endpointErr *proviantErrors.ReceiptEndpointError
		if !errors.As(err, &endpointErr) {
			t.Errorf("err = %v (%T), want *ReceiptEndpointError", err, err)
		}
	})

	t.Run("content returned", func(t *testing.T) {
		content, err := ctrl.readPlainChatResponse([]byte(`{"choices":[{"message":{"content":"hello"}}]}`), time.Now())
		if err != nil {
			t.Fatalf("readPlainChatResponse() = %v", err)
		}
		if content != "hello" {
			t.Errorf("content = %q, want %q", content, "hello")
		}
	})
}

func TestStreamChatResponseVariants(t *testing.T) {
	logger := zerolog.Nop()
	ctrl := ReceiptScanControllerImpl{Logger: &logger}

	t.Run("mid-stream endpoint error", func(t *testing.T) {
		stream := strings.NewReader("data: {\"error\":\"aborted\"}\n\n")
		_, err := ctrl.streamChatResponse(stream, time.Now())
		var endpointErr *proviantErrors.ReceiptEndpointError
		if !errors.As(err, &endpointErr) {
			t.Fatalf("err = %v (%T), want *ReceiptEndpointError", err, err)
		}
		if !strings.Contains(err.Error(), "aborted") {
			t.Errorf("err = %v, want the endpoint message", err)
		}
	})

	t.Run("unparsable data line is skipped", func(t *testing.T) {
		stream := strings.NewReader("data: not-json\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		content, err := ctrl.streamChatResponse(stream, time.Now())
		if err != nil {
			t.Fatalf("streamChatResponse() = %v", err)
		}
		if content != "ok" {
			t.Errorf("content = %q, want %q (bad line skipped)", content, "ok")
		}
	})

	t.Run("non-data lines are ignored", func(t *testing.T) {
		stream := strings.NewReader(": comment\n\nevent: message\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n")
		content, err := ctrl.streamChatResponse(stream, time.Now())
		if err != nil {
			t.Fatalf("streamChatResponse() = %v", err)
		}
		if content != "x" {
			t.Errorf("content = %q, want %q", content, "x")
		}
	})

	t.Run("empty deltas are skipped", func(t *testing.T) {
		stream := strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\ndata: {\"choices\":[{\"delta\":{}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"tail\"}}]}\n\n")
		content, err := ctrl.streamChatResponse(stream, time.Now())
		if err != nil {
			t.Fatalf("streamChatResponse() = %v", err)
		}
		if content != "tail" {
			t.Errorf("content = %q, want %q", content, "tail")
		}
	})

	t.Run("empty stream returns empty content", func(t *testing.T) {
		content, err := ctrl.streamChatResponse(strings.NewReader(""), time.Now())
		if err != nil {
			t.Fatalf("streamChatResponse() = %v", err)
		}
		if content != "" {
			t.Errorf("content = %q, want empty", content)
		}
	})
}

func TestTruncateForLog(t *testing.T) {
	t.Run("short payload unchanged", func(t *testing.T) {
		if got := truncateForLog([]byte("short"), 500); got != "short" {
			t.Errorf("truncateForLog() = %q, want unchanged", got)
		}
	})

	t.Run("long payload truncated with marker", func(t *testing.T) {
		payload := bytes.Repeat([]byte("a"), 1000)
		got := truncateForLog(payload, 500)
		if len(got) <= 500 || !strings.Contains(got, "truncated, 1000 bytes total") {
			t.Errorf("truncateForLog() = %d bytes, want a truncation marker", len(got))
		}
	})

	t.Run("multibyte runes are not split", func(t *testing.T) {
		payload := []byte(strings.Repeat("€", 100)) // 300 bytes
		got := truncateForLog(payload, 100)
		if !utf8ValidString(got) {
			t.Errorf("truncateForLog() split a multi-byte rune: %q", got)
		}
		if !strings.Contains(got, "truncated") {
			t.Errorf("truncateForLog() = %q, want a truncation marker", got)
		}
	})
}

func utf8ValidString(s string) bool {
	for _, r := range s {
		if r == 0xFFFD {
			return false
		}
	}
	return true
}

func TestTruncateUTF8(t *testing.T) {
	t.Run("short string unchanged", func(t *testing.T) {
		if got := truncateUTF8("abc", 10); got != "abc" {
			t.Errorf("truncateUTF8() = %q, want unchanged", got)
		}
	})

	t.Run("ascii truncated at byte boundary", func(t *testing.T) {
		if got := truncateUTF8("abcdef", 3); got != "abc" {
			t.Errorf("truncateUTF8() = %q, want %q", got, "abc")
		}
	})

	t.Run("multibyte rune not split", func(t *testing.T) {
		// Each euro sign is 3 bytes; a 4-byte budget fits exactly one.
		if got := truncateUTF8("€€", 4); got != "€" {
			t.Errorf("truncateUTF8() = %q, want one euro sign", got)
		}
	})

	t.Run("exact fit unchanged", func(t *testing.T) {
		if got := truncateUTF8("€€", 6); got != "€€" {
			t.Errorf("truncateUTF8() = %q, want unchanged", got)
		}
	})
}

func TestSalvageReceiptItems(t *testing.T) {
	t.Run("payload without item start yields nothing", func(t *testing.T) {
		if got := salvageReceiptItems(`{"items":[]}`); got != nil {
			t.Errorf("salvageReceiptItems() = %+v, want nil", got)
		}
	})

	t.Run("truncated payload salvages complete and partial items", func(t *testing.T) {
		// The second item is cut off after its last complete field, so the
		// salvage path can close the object; a cut inside a string cannot.
		items := salvageReceiptItems(`{"items":[{"name":"Milk","amount":1},{"name":"Bread","amount":2`)
		if len(items) != 2 {
			t.Fatalf("items = %+v, want 2 salvaged items", items)
		}
		if items[0].Name != "Milk" || items[0].Amount != 1 {
			t.Errorf("items[0] = %+v, want Milk x1", items[0])
		}
		if items[1].Name != "Bread" || items[1].Amount != 2 {
			t.Errorf("items[1] = %+v, want the truncated Bread x2", items[1])
		}
	})

	t.Run("item cut inside a string value is dropped", func(t *testing.T) {
		items := salvageReceiptItems(`{"items":[{"name":"Milk","amount":1},{"name":"Brea`)
		if len(items) != 1 || items[0].Name != "Milk" {
			t.Errorf("items = %+v, want only the complete Milk item", items)
		}
	})

	t.Run("bracket inside a product name is not treated as array end", func(t *testing.T) {
		items := salvageReceiptItems(`{"name":"Milk [1L]","amount":1}]}`)
		if len(items) != 1 || items[0].Name != "Milk [1L]" {
			t.Errorf("items = %+v, want one %q", items, "Milk [1L]")
		}
	})

	t.Run("broken chunks are skipped", func(t *testing.T) {
		items := salvageReceiptItems(`{"name":"Milk","amount":1},{"name":,"amount":2}`)
		if len(items) != 1 || items[0].Name != "Milk" {
			t.Errorf("items = %+v, want only the parseable Milk item", items)
		}
	})
}

func TestIndexJSONBracketOutsideString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "bracket outside string", input: `a]b`, want: 1},
		{name: "bracket inside string ignored", input: `"a]b"`, want: -1},
		{name: "escaped quote keeps string open", input: `"a\"]b"`, want: -1},
		{name: "bracket after closed string", input: `"a"]`, want: 3},
		{name: "no bracket", input: `abc`, want: -1},
		{name: "empty", input: ``, want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := indexJSONBracketOutsideString(tt.input); got != tt.want {
				t.Errorf("indexJSONBracketOutsideString(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildReceiptItems(t *testing.T) {
	price := func(v float64) *float64 { return &v }

	t.Run("clamping and validation", func(t *testing.T) {
		items := buildReceiptItems([]tolerantReceiptItem{
			{Name: "  Milk  ", Amount: 2.4, Unit: " l ", Price: price(1.29)},
			{Name: "", Amount: 1},                          // empty name dropped
			{Name: "Zero", Amount: 0, Price: price(-5)},    // amount clamped to 1, negative price dropped
			{Name: "Huge", Amount: 1e9, Price: price(1e9)}, // amount and price clamped/dropped
			{Name: strings.Repeat("n", 500), Amount: 1},
			{Name: "Unit", Amount: 1, Unit: strings.Repeat("u", 50)},
		})
		if len(items) != 5 {
			t.Fatalf("items = %d, want 5 (empty name dropped)", len(items))
		}
		if items[0].Name != "Milk" || items[0].Amount != 2 || items[0].Unit != "l" {
			t.Errorf("items[0] = %+v, want trimmed Milk x2 l", items[0])
		}
		if items[0].Price == nil || *items[0].Price != 1.29 {
			t.Errorf("items[0].Price = %v, want 1.29", items[0].Price)
		}
		if items[1].Amount != 1 || items[1].Price != nil {
			t.Errorf("items[1] = %+v, want amount clamped to 1 and negative price dropped", items[1])
		}
		if items[2].Amount != util.ReceiptItemMaxAmount || items[2].Price != nil {
			t.Errorf("items[2] = %+v, want amount clamped to %d and oversized price dropped", items[2], util.ReceiptItemMaxAmount)
		}
		if len(items[3].Name) > util.ReceiptItemMaxNameLength {
			t.Errorf("name length = %d, want <= %d", len(items[3].Name), util.ReceiptItemMaxNameLength)
		}
		if len(items[4].Unit) > util.ReceiptItemMaxUnitLength {
			t.Errorf("unit length = %d, want <= %d", len(items[4].Unit), util.ReceiptItemMaxUnitLength)
		}
	})

	t.Run("nil input yields empty", func(t *testing.T) {
		if got := buildReceiptItems(nil); len(got) != 0 {
			t.Errorf("buildReceiptItems(nil) = %+v, want empty", got)
		}
	})
}

func TestExtractReceiptJSONPayloadsUnterminated(t *testing.T) {
	t.Run("unterminated object is appended as-is", func(t *testing.T) {
		payloads := extractReceiptJSONPayloads(`{"items":[`)
		if len(payloads) != 1 || payloads[0] != `{"items":[` {
			t.Errorf("payloads = %q, want the unterminated object", payloads)
		}
	})

	t.Run("no object yields nil", func(t *testing.T) {
		if payloads := extractReceiptJSONPayloads("no json here"); payloads != nil {
			t.Errorf("payloads = %q, want nil", payloads)
		}
	})

	t.Run("nested objects captured whole", func(t *testing.T) {
		payloads := extractReceiptJSONPayloads(`{"a":{"b":1}} trailing {"c":2}`)
		if len(payloads) != 2 {
			t.Fatalf("payloads = %q, want 2 objects", payloads)
		}
		if payloads[0] != `{"a":{"b":1}}` || payloads[1] != `{"c":2}` {
			t.Errorf("payloads = %q, want the two top-level objects", payloads)
		}
	})

	t.Run("escaped quote inside string does not close it", func(t *testing.T) {
		payloads := extractReceiptJSONPayloads(`{"name":"a\"b"} {"x":1}`)
		if len(payloads) != 2 || payloads[0] != `{"name":"a\"b"}` {
			t.Errorf("payloads = %q, want the escaped-quote object captured whole", payloads)
		}
	})
}

func TestParseReceiptItemsSalvageFallback(t *testing.T) {
	// Token-cap truncated output: strict parse and repair both fail, so the
	// salvage path recovers the complete item and closes the dangling one.
	items, err := ParseReceiptItems(`{"items":[{"name":"Milk","amount":1},{"name":"Bread","amount":2`)
	if err != nil {
		t.Fatalf("ParseReceiptItems() = %v", err)
	}
	if len(items) != 2 || items[0].Name != "Milk" || items[1].Name != "Bread" {
		t.Errorf("items = %+v, want salvaged [Milk Bread]", items)
	}
}
