package controllers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	dbController "github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestOffDumpRepository(t *testing.T) *dbController.OffDumpRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "off_dump.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open dump database: %v", err)
	}
	repo := dbController.NewOffDumpRepository(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("failed to migrate dump database: %v", err)
	}
	return repo
}

// gzipJSONL encodes products as a gzip-compressed stream of JSON lines, the
// shape of the OFF dump and delta files.
func gzipJSONL(t *testing.T, products []map[string]any) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	for _, product := range products {
		encoded, err := json.Marshal(product)
		if err != nil {
			t.Fatalf("marshal product: %v", err)
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			t.Fatalf("write product: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buffer.Bytes()
}

// gzipRawJSONL encodes already-serialized lines, so a test can inject a line
// that is not valid JSON at all.
func gzipRawJSONL(t *testing.T, lines ...string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	for _, line := range lines {
		if _, err := writer.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("write line: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buffer.Bytes()
}

type offDumpServer struct {
	server        *httptest.Server
	dump          []byte
	lastMod       string
	etag          string
	ignoreRange   bool
	truncateDump  bool
	index         string
	deltaFiles    map[string][]byte
	dumpRequests  int
	rangeRequests []string
}

func newOffDumpServer(t *testing.T, dump []byte, lastMod string) *offDumpServer {
	t.Helper()
	fixture := &offDumpServer{dump: dump, lastMod: lastMod, deltaFiles: map[string][]byte{}}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/dump.jsonl.gz":
			fixture.serveDump(w, r)
		case r.URL.Path == "/delta/index.txt":
			_, _ = w.Write([]byte(fixture.index))
		case strings.HasPrefix(r.URL.Path, "/delta/"):
			name := strings.TrimPrefix(r.URL.Path, "/delta/")
			body, ok := fixture.deltaFiles[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

// serveDump emulates the OFF/S3 object: byte ranges with an If-Range validator,
// so a resumed download can be exercised.
func (f *offDumpServer) serveDump(w http.ResponseWriter, r *http.Request) {
	f.dumpRequests++
	if f.lastMod != "" {
		w.Header().Set("Last-Modified", f.lastMod)
	}
	if f.etag != "" {
		w.Header().Set("ETag", f.etag)
	}
	w.Header().Set("Accept-Ranges", "bytes")

	if f.truncateDump {
		w.Header().Set("Content-Length", strconv.Itoa(len(f.dump)))
		_, _ = w.Write(f.dump[:len(f.dump)/2])
		return
	}

	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" || f.ignoreRange {
		_, _ = w.Write(f.dump)
		return
	}
	f.rangeRequests = append(f.rangeRequests, rangeHeader)
	if ifRange := r.Header.Get("If-Range"); ifRange != "" && ifRange != f.etag {
		_, _ = w.Write(f.dump)
		return
	}
	start, ok := rangeStart(rangeHeader)
	if !ok || start >= int64(len(f.dump)) {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(f.dump)-1, len(f.dump)))
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(f.dump[start:])
}

func rangeStart(header string) (int64, bool) {
	value := strings.TrimSuffix(strings.TrimPrefix(header, "bytes="), "-")
	start, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return start, true
}

func (f *offDumpServer) controller(t *testing.T, repo *dbController.OffDumpRepository, config *configuration.OpenFoodFactsConfiguration) *OffDumpController {
	t.Helper()
	logger := zerolog.Nop()
	config.DumpURL = f.server.URL + "/dump.jsonl.gz"
	config.DumpStagingPath = t.TempDir()
	controller := NewOffDumpController(&logger, config, repo)
	controller.DeltaIndexURL = f.server.URL + "/delta/index.txt"
	controller.DeltaBaseURL = f.server.URL + "/delta/"
	controller.HTTPClient = f.server.Client()
	return controller
}

// TestOffDumpControllerFullImport verifies a fresh store downloads and imports
// the full dump, records state, and serves lookups without a second download.
func TestOffDumpControllerFullImport(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{
		{"code": "111", "product_name": "Milk", "categories": "Dairy", "ecoscore_data": map[string]any{"agribalyse": map[string]any{"co2_total": 1.5}}},
		{"code": "222", "product_name": "Bread"},
		{"code": "", "product_name": "No barcode is skipped"},
	})
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	t.Run("all barcoded products are stored", func(t *testing.T) {
		count, err := repo.Count()
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 2 {
			t.Errorf("Count = %d, want 2", count)
		}
	})

	t.Run("lookup returns mapped product", func(t *testing.T) {
		product, found := controller.LookupProduct("111")
		if !found {
			t.Fatalf("found = false, want true")
		}
		if product.ProductName != "Milk" || product.Categories != "Dairy" {
			t.Errorf("product = %+v, want Milk/Dairy", product)
		}
		if product.CO2KgPerKg == nil || *product.CO2KgPerKg != 1.5 {
			t.Errorf("co2 = %v, want 1.5", product.CO2KgPerKg)
		}
	})

	t.Run("lookup miss reports not found", func(t *testing.T) {
		if _, found := controller.LookupProduct("999"); found {
			t.Errorf("found = true, want false")
		}
	})

	t.Run("full import timestamp is recorded", func(t *testing.T) {
		value, err := repo.GetState(util.OffDumpStateLastFullImport)
		if err != nil {
			t.Fatalf("GetState: %v", err)
		}
		if value == "" {
			t.Errorf("last full import timestamp is empty")
		}
	})
}

// TestOffDumpControllerStagingCleanup verifies a completed full import removes
// the staged file and clears the resumable state.
func TestOffDumpControllerStagingCleanup(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	_, readyPath := controller.stagingPaths()
	if _, err := os.Stat(readyPath); !os.IsNotExist(err) {
		t.Errorf("staged file still present: %v", err)
	}
	keys := []string{
		util.OffDumpStateFullImportURL,
		util.OffDumpStateFullImportETag,
		util.OffDumpStateFullImportLastModified,
		util.OffDumpStateFullImportRecords,
	}
	for _, key := range keys {
		value, err := repo.GetState(key)
		if err != nil {
			t.Fatalf("GetState(%s): %v", key, err)
		}
		if value != "" {
			t.Errorf("state %s = %q, want empty", key, value)
		}
	}
}

// TestOffDumpControllerResumeDownload verifies a partial staged download is
// resumed with a byte-range request instead of restarting.
func TestOffDumpControllerResumeDownload(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{
		{"code": "111", "product_name": "Milk"},
		{"code": "222", "product_name": "Bread"},
	})
	fixture := newOffDumpServer(t, dump, "")
	fixture.etag = `"v1"`
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	partPath, _ := controller.stagingPaths()
	half := len(dump) / 2
	if err := os.WriteFile(partPath, dump[:half], 0o600); err != nil {
		t.Fatalf("seed partial download: %v", err)
	}
	if err := repo.SetState(util.OffDumpStateFullImportURL, controller.dumpURL()); err != nil {
		t.Fatalf("seed URL state: %v", err)
	}
	if err := repo.SetState(util.OffDumpStateFullImportETag, fixture.etag); err != nil {
		t.Fatalf("seed ETag state: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	t.Run("resumes with a range request", func(t *testing.T) {
		want := fmt.Sprintf("bytes=%d-", half)
		if len(fixture.rangeRequests) != 1 || fixture.rangeRequests[0] != want {
			t.Errorf("range requests = %v, want [%s]", fixture.rangeRequests, want)
		}
	})

	t.Run("imports the complete dump", func(t *testing.T) {
		count, err := repo.Count()
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 2 {
			t.Errorf("Count = %d, want 2", count)
		}
	})
}

// TestOffDumpControllerInterruptedDownloadResumes verifies a truncated download
// keeps its partial file and resumes on the next run instead of restarting.
func TestOffDumpControllerInterruptedDownloadResumes(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "")
	fixture.etag = `"v1"`
	fixture.truncateDump = true
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	if err := controller.RunOnce(context.Background()); err == nil {
		t.Fatalf("RunOnce = nil, want error for a truncated download")
	}

	partPath, _ := controller.stagingPaths()
	size, err := fileSize(partPath)
	if err != nil {
		t.Fatalf("fileSize: %v", err)
	}
	if size != int64(len(dump)/2) {
		t.Errorf("partial size = %d, want %d", size, len(dump)/2)
	}
	if value, err := repo.GetState(util.OffDumpStateLastFullImport); err != nil {
		t.Fatalf("GetState: %v", err)
	} else if value != "" {
		t.Errorf("last full import = %q, want empty after a truncated download", value)
	}

	fixture.truncateDump = false
	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("resumed RunOnce: %v", err)
	}
	product, found := controller.LookupProduct("111")
	if !found || product.ProductName != "Milk" {
		t.Errorf("lookup 111 = %+v, %v; want Milk, true", product, found)
	}
}

// TestOffDumpControllerStagingETagChanged verifies a partial download of a
// superseded object is discarded rather than stitched onto the new dump.
func TestOffDumpControllerStagingETagChanged(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "999", "product_name": "New"}})
	fixture := newOffDumpServer(t, dump, "")
	fixture.etag = `"v2"`
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	partPath, _ := controller.stagingPaths()
	if err := os.WriteFile(partPath, []byte("stale partial bytes"), 0o600); err != nil {
		t.Fatalf("seed partial download: %v", err)
	}
	if err := repo.SetState(util.OffDumpStateFullImportURL, controller.dumpURL()); err != nil {
		t.Fatalf("seed URL state: %v", err)
	}
	if err := repo.SetState(util.OffDumpStateFullImportETag, `"v1"`); err != nil {
		t.Fatalf("seed ETag state: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if _, found := controller.LookupProduct("999"); !found {
		t.Errorf("lookup 999 found = false, want true")
	}
}

// TestOffDumpControllerNoRangeSupport verifies a server that ignores range
// requests still imports correctly from scratch.
func TestOffDumpControllerNoRangeSupport(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "")
	fixture.etag = `"v1"`
	fixture.ignoreRange = true
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	partPath, _ := controller.stagingPaths()
	if err := os.WriteFile(partPath, []byte("stale partial bytes"), 0o600); err != nil {
		t.Fatalf("seed partial download: %v", err)
	}
	if err := repo.SetState(util.OffDumpStateFullImportURL, controller.dumpURL()); err != nil {
		t.Fatalf("seed URL state: %v", err)
	}
	if err := repo.SetState(util.OffDumpStateFullImportETag, fixture.etag); err != nil {
		t.Fatalf("seed ETag state: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	product, found := controller.LookupProduct("111")
	if !found || product.ProductName != "Milk" {
		t.Errorf("lookup 111 = %+v, %v; want Milk, true", product, found)
	}
}

// TestOffDumpControllerResumeImport verifies a staged download left by an
// interrupted import is imported without re-downloading.
func TestOffDumpControllerResumeImport(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{
		{"code": "", "product_name": "barcodeless record consumed before the checkpoint"},
		{"code": "111", "product_name": "Milk"},
		{"code": "222", "product_name": "Bread"},
	})
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	_, readyPath := controller.stagingPaths()
	if err := os.WriteFile(readyPath, dump, 0o600); err != nil {
		t.Fatalf("seed staged download: %v", err)
	}
	if err := repo.SetState(util.OffDumpStateFullImportRecords, "1"); err != nil {
		t.Fatalf("seed records state: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if fixture.dumpRequests != 0 {
		t.Errorf("dump requests = %d, want 0", fixture.dumpRequests)
	}
	for _, barcode := range []string{"111", "222"} {
		if _, found := controller.LookupProduct(barcode); !found {
			t.Errorf("lookup %s found = false, want true", barcode)
		}
	}
	records, err := repo.GetState(util.OffDumpStateFullImportRecords)
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if records != "" {
		t.Errorf("records = %q, want empty", records)
	}
}

// TestOffDumpControllerStagedImportSkip verifies the import skips the recorded
// consumed records so a resume does not re-upsert the prefix.
func TestOffDumpControllerStagedImportSkip(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{
		{"code": "111", "product_name": "Milk"},
		{"code": "222", "product_name": "Bread"},
		{"code": "333", "product_name": "Butter"},
	})
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled: true,
	})

	_, readyPath := controller.stagingPaths()
	if err := os.WriteFile(readyPath, dump, 0o600); err != nil {
		t.Fatalf("seed staged download: %v", err)
	}

	if _, err := controller.importStagedDump(context.Background(), readyPath, 2); err != nil {
		t.Fatalf("importStagedDump: %v", err)
	}

	if _, found := controller.LookupProduct("111"); found {
		t.Errorf("lookup 111 found = true, want false (consumed before the checkpoint)")
	}
	if _, found := controller.LookupProduct("222"); found {
		t.Errorf("lookup 222 found = true, want false (consumed before the checkpoint)")
	}
	if _, found := controller.LookupProduct("333"); !found {
		t.Errorf("lookup 333 found = false, want true")
	}
}

// TestOffDumpControllerSkipsMalformedRecords verifies a product the decoder
// cannot read is skipped instead of aborting the whole import, and that a
// string co2_total is read rather than rejected.
func TestOffDumpControllerSkipsMalformedRecords(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{
		{"code": "111", "product_name": "Milk"},
		{"code": map[string]any{"unexpected": "object"}, "product_name": "Unreadable barcode"},
		{"code": "222", "product_name": "Bread", "ecoscore_data": map[string]any{"agribalyse": map[string]any{"co2_total": "1.5"}}},
	})
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled: true,
	})

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	count, err := repo.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}

	t.Run("string co2_total is read", func(t *testing.T) {
		product, found := controller.LookupProduct("222")
		if !found {
			t.Fatalf("lookup 222 found = false, want true")
		}
		if product.CO2KgPerKg == nil || *product.CO2KgPerKg != 1.5 {
			t.Errorf("co2 = %v, want 1.5", product.CO2KgPerKg)
		}
	})

	t.Run("import still completes", func(t *testing.T) {
		value, err := repo.GetState(util.OffDumpStateLastFullImport)
		if err != nil {
			t.Fatalf("GetState: %v", err)
		}
		if value == "" {
			t.Errorf("last full import is empty, want a completed import despite the malformed record")
		}
	})
}

// TestOffDumpControllerSkipsInvalidJSONLines verifies a syntactically invalid
// line is skipped rather than aborting the whole import.
func TestOffDumpControllerSkipsInvalidJSONLines(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipRawJSONL(t,
		`{"code":"111","product_name":"Milk"}`,
		`{"code":"222", this is not json`,
		`{"code":"333","product_name":"Butter"}`,
	)
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled: true,
	})

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	count, err := repo.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2 (invalid line skipped)", count)
	}
	if _, found := controller.LookupProduct("333"); !found {
		t.Errorf("lookup 333 found = false, want true (import continues past the invalid line)")
	}
	value, err := repo.GetState(util.OffDumpStateLastFullImport)
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if value == "" {
		t.Errorf("last full import is empty, want a completed import despite the invalid line")
	}
}

// TestOffDumpControllerAcceptsNumericAndPaddedBarcode verifies a numeric code is
// stored under its barcode and padded whitespace is trimmed from the stored row.
func TestOffDumpControllerAcceptsNumericAndPaddedBarcode(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipRawJSONL(t,
		`{"code":1234567890123,"product_name":"Numeric"}`,
		`{"code":"  4006381333931  ","product_name":"Padded"}`,
	)
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled: true,
	})

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	t.Run("numeric barcode is stored", func(t *testing.T) {
		product, found := controller.LookupProduct("1234567890123")
		if !found || product.ProductName != "Numeric" {
			t.Errorf("lookup 1234567890123 = %+v, %v; want Numeric, true", product, found)
		}
	})

	t.Run("padded barcode is trimmed", func(t *testing.T) {
		product, found := controller.LookupProduct("4006381333931")
		if !found {
			t.Fatalf("lookup 4006381333931 found = false, want true")
		}
		if product.Barcode != "4006381333931" {
			t.Errorf("Barcode = %q, want the trimmed value", product.Barcode)
		}
	})
}

// TestOffDumpControllerRecoversFromCorruptStagedDump verifies an unreadable
// staged file is discarded so the next run re-downloads instead of re-failing
// forever.
func TestOffDumpControllerRecoversFromCorruptStagedDump(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	_, readyPath := controller.stagingPaths()
	if err := os.WriteFile(readyPath, []byte("not a gzip stream"), 0o600); err != nil {
		t.Fatalf("seed corrupt staged download: %v", err)
	}
	if err := repo.SetState(util.OffDumpStateFullImportURL, controller.dumpURL()); err != nil {
		t.Fatalf("seed URL state: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err == nil {
		t.Fatalf("RunOnce = nil, want an error for a corrupt staged dump")
	}
	if _, err := os.Stat(readyPath); !os.IsNotExist(err) {
		t.Errorf("corrupt staged file still present: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("recovery RunOnce: %v", err)
	}
	product, found := controller.LookupProduct("111")
	if !found || product.ProductName != "Milk" {
		t.Errorf("lookup 111 = %+v, %v; want Milk, true", product, found)
	}
}

// TestOffDumpControllerDiscardsStagedDumpFromOtherURL verifies a staged download
// from a different dump URL is discarded instead of imported.
func TestOffDumpControllerDiscardsStagedDumpFromOtherURL(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	_, readyPath := controller.stagingPaths()
	if err := os.WriteFile(readyPath, []byte("staged from another mirror"), 0o600); err != nil {
		t.Fatalf("seed staged download: %v", err)
	}
	if err := repo.SetState(util.OffDumpStateFullImportURL, "https://other.example/dump.jsonl.gz"); err != nil {
		t.Fatalf("seed URL state: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if fixture.dumpRequests != 1 {
		t.Errorf("dump requests = %d, want 1 (stage from another URL discarded)", fixture.dumpRequests)
	}
	product, found := controller.LookupProduct("111")
	if !found || product.ProductName != "Milk" {
		t.Errorf("lookup 111 = %+v, %v; want Milk, true", product, found)
	}
}

// TestOffDumpControllerDiscardsExpiredStagedDump verifies a staged download older
// than the expiry is discarded so a fresh dump is downloaded.
func TestOffDumpControllerDiscardsExpiredStagedDump(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	_, readyPath := controller.stagingPaths()
	if err := os.WriteFile(readyPath, []byte("staged long ago"), 0o600); err != nil {
		t.Fatalf("seed staged download: %v", err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	if err := os.Chtimes(readyPath, old, old); err != nil {
		t.Fatalf("age staged download: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if fixture.dumpRequests != 1 {
		t.Errorf("dump requests = %d, want 1 (expired stage discarded)", fixture.dumpRequests)
	}
	product, found := controller.LookupProduct("111")
	if !found || product.ProductName != "Milk" {
		t.Errorf("lookup 111 = %+v, %v; want Milk, true", product, found)
	}
}

// TestOffDumpControllerHoldsBaselineForUnreadableDelta verifies a delta whose
// every record is unreadable does not advance the baseline, so the change is not
// silently lost.
func TestOffDumpControllerHoldsBaselineForUnreadableDelta(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "Thu, 01 Jan 1970 00:20:00 GMT")
	fixture.deltaFiles["openfoodfacts_products_1200_2000.json.gz"] = gzipRawJSONL(t, "not json", "also not json")
	fixture.index = "openfoodfacts_products_1200_2000.json.gz\n"

	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("full RunOnce: %v", err)
	}
	if err := controller.RunOnce(context.Background()); err == nil {
		t.Fatalf("delta RunOnce = nil, want an error for an unreadable delta")
	}

	value, err := repo.GetState(util.OffDumpStateLastDeltaEnd)
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if value != "1200" {
		t.Errorf("last delta end = %q, want the unchanged full-import baseline 1200", value)
	}
}

// TestOffDumpControllerSkipsDeltaAfterRepeatedFailures verifies an unreadable
// delta is retried a bounded number of times and then skipped, so it cannot
// stall every newer delta indefinitely.
func TestOffDumpControllerSkipsDeltaAfterRepeatedFailures(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "Thu, 01 Jan 1970 00:20:00 GMT")
	fixture.deltaFiles["openfoodfacts_products_1200_2000.json.gz"] = gzipRawJSONL(t, "not json", "also not json")
	fixture.index = "openfoodfacts_products_1200_2000.json.gz\n"

	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("full RunOnce: %v", err)
	}
	for attempt := 1; attempt < maxDeltaAttempts; attempt++ {
		if err := controller.RunOnce(context.Background()); err == nil {
			t.Fatalf("delta RunOnce attempt %d = nil, want a held-baseline error", attempt)
		}
	}
	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("delta RunOnce after %d attempts: %v", maxDeltaAttempts, err)
	}

	value, err := repo.GetState(util.OffDumpStateLastDeltaEnd)
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if value != "2000" {
		t.Errorf("last delta end = %q, want 2000 after skipping the unreadable delta", value)
	}
}

// TestOffDumpControllerDiscardsStagedDumpOfUnknownOrigin verifies a staged dump
// with no recorded URL and no import in progress is discarded and re-downloaded
// rather than imported from an untrusted origin.
func TestOffDumpControllerDiscardsStagedDumpOfUnknownOrigin(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "")
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	_, readyPath := controller.stagingPaths()
	if err := os.WriteFile(readyPath, dump, 0o600); err != nil {
		t.Fatalf("seed staged download without provenance: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if fixture.dumpRequests != 1 {
		t.Errorf("dump requests = %d, want 1 (unknown-origin stage discarded)", fixture.dumpRequests)
	}
	product, found := controller.LookupProduct("111")
	if !found || product.ProductName != "Milk" {
		t.Errorf("lookup 111 = %+v, %v; want Milk, true", product, found)
	}
}

// TestOffDumpControllerResetsStaleCheckpointOnDownloadFailure verifies a stale
// checkpoint is cleared before a fresh download, so a later .ready cannot skip
// that many products.
func TestOffDumpControllerResetsStaleCheckpointOnDownloadFailure(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "")
	fixture.truncateDump = true
	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	if err := repo.SetState(util.OffDumpStateFullImportRecords, "5"); err != nil {
		t.Fatalf("seed stale checkpoint: %v", err)
	}

	if err := controller.RunOnce(context.Background()); err == nil {
		t.Fatalf("RunOnce = nil, want a truncated-download error")
	}

	value, err := repo.GetState(util.OffDumpStateFullImportRecords)
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if value != "0" {
		t.Errorf("checkpoint = %q, want 0 (a stale count must not survive a fresh download)", value)
	}
}

// TestOffDumpControllerBoundsSmallFetches verifies a stalled delta index fetch
// fails on the small-fetch timeout instead of hanging the run.
func TestOffDumpControllerBoundsSmallFetches(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)

	logger := zerolog.Nop()
	controller := NewOffDumpController(&logger, &configuration.OpenFoodFactsConfiguration{}, repo)
	controller.DeltaIndexURL = server.URL + "/index.txt"
	controller.smallFetchTimeout = 50 * time.Millisecond

	if _, err := controller.fetchDeltaIndex(context.Background()); err == nil {
		t.Fatal("fetchDeltaIndex = nil error, want the small-fetch timeout to fire")
	}
}

// TestOffDumpControllerDeltaImport verifies that a full import followed by an
// expired-tick run applies only the deltas published since the dump.
func TestOffDumpControllerDeltaImport(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "Thu, 01 Jan 1970 00:20:00 GMT") // unix 1200
	fixture.deltaFiles["openfoodfacts_products_1200_2000.json.gz"] = gzipJSONL(t, []map[string]any{
		{"code": "111", "product_name": "Oat Milk"},
		{"code": "333", "product_name": "Butter"},
	})
	fixture.index = "openfoodfacts_products_1200_2000.json.gz\n"

	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  0,
	})

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("full RunOnce: %v", err)
	}
	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("delta RunOnce: %v", err)
	}

	t.Run("delta updates existing and adds new", func(t *testing.T) {
		updated, found := controller.LookupProduct("111")
		if !found || updated.ProductName != "Oat Milk" {
			t.Errorf("lookup 111 = %+v, %v; want Oat Milk, true", updated, found)
		}
		if _, found := controller.LookupProduct("333"); !found {
			t.Errorf("lookup 333 found = false, want true")
		}
	})

	t.Run("delta baseline advances", func(t *testing.T) {
		value, err := repo.GetState(util.OffDumpStateLastDeltaEnd)
		if err != nil {
			t.Fatalf("GetState: %v", err)
		}
		if value != "2000" {
			t.Errorf("last delta end = %q, want 2000", value)
		}
	})
}

// TestOffDumpControllerDeltaWindow verifies deltas older than the configured
// window are not applied.
func TestOffDumpControllerDeltaWindow(t *testing.T) {
	repo := newTestOffDumpRepository(t)
	dump := gzipJSONL(t, []map[string]any{{"code": "111", "product_name": "Milk"}})
	fixture := newOffDumpServer(t, dump, "Thu, 01 Jan 1970 00:20:00 GMT")
	fixture.deltaFiles["openfoodfacts_products_1200_2000.json.gz"] = gzipJSONL(t, []map[string]any{
		{"code": "111", "product_name": "Oat Milk"},
	})
	fixture.index = "openfoodfacts_products_1200_2000.json.gz\n"

	controller := fixture.controller(t, repo, &configuration.OpenFoodFactsConfiguration{
		DumpImportEnabled:    true,
		DumpImportExpiryDays: 7,
		DumpDeltaWindowDays:  14,
	})

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("full RunOnce: %v", err)
	}
	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("delta RunOnce: %v", err)
	}

	product, found := controller.LookupProduct("111")
	if !found || product.ProductName != "Milk" {
		t.Errorf("lookup 111 = %+v, %v; want unchanged Milk, true", product, found)
	}
}

// TestParseDeltaFilename verifies the OFF delta filename contract.
func TestParseDeltaFilename(t *testing.T) {
	t.Run("valid filename", func(t *testing.T) {
		delta, ok := parseDeltaFilename("openfoodfacts_products_1792217949_1792304521.json.gz")
		if !ok {
			t.Fatalf("ok = false, want true")
		}
		if delta.startAt != 1792217949 || delta.endAt != 1792304521 {
			t.Errorf("delta = %+v, want start 1792217949 end 1792304521", delta)
		}
	})

	t.Run("invalid filenames are rejected", func(t *testing.T) {
		for _, name := range []string{"", "index.txt", "openfoodfacts_products_abc.json.gz", "other_products_1_2.json.gz"} {
			if _, ok := parseDeltaFilename(name); ok {
				t.Errorf("parseDeltaFilename(%q) = ok, want false", name)
			}
		}
	})
}

// The staged download is named after the dump store, so a store pointed at a
// different database resumes its own .part/.ready files instead of the default's.
func TestOffDumpControllerStagingPathsFollowDumpDatabase(t *testing.T) {
	stagingDir := t.TempDir()
	logger := zerolog.Nop()
	controller := NewOffDumpController(&logger, &configuration.OpenFoodFactsConfiguration{
		DumpDatabase:    "proviant_off_dump",
		DumpStagingPath: stagingDir,
	}, nil)

	partPath, readyPath := controller.stagingPaths()
	base := filepath.Join(stagingDir, "proviant_off_dump"+util.OffDumpStagingSuffix)
	if partPath != base+util.OffDumpStagingPartSuffix {
		t.Errorf("partPath = %q, want %q", partPath, base+util.OffDumpStagingPartSuffix)
	}
	if readyPath != base+util.OffDumpStagingReadySuffix {
		t.Errorf("readyPath = %q, want %q", readyPath, base+util.OffDumpStagingReadySuffix)
	}
}
