package controllers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dbController "github.com/Isotop7/proviant/controllers/database"
	proviantErrors "github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/models/external"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
)

// errDumpStreamUnreadable marks a dump stream whose bytes cannot be decoded, so
// the staged file can be discarded and re-downloaded instead of re-failing on
// every run.
var errDumpStreamUnreadable = errors.New("OpenFoodFacts dump stream is unreadable")

// offDumpReadBufferSize is the initial read buffer for one dump line. Lines are
// read with ReadBytes, so this grows without a hard limit: a very large product
// record is buffered whole rather than aborting the import.
const offDumpReadBufferSize = 1 << 20

// offDumpProgressInterval is how many downloaded bytes pass between progress
// log lines.
const offDumpProgressInterval = 256 << 20

// maxDeltaAttempts bounds how many runs a delta may fail to import before its
// baseline advances anyway, so one permanently unreadable delta cannot stall
// newer deltas until the next full import.
const maxDeltaAttempts = 3

// offDumpSmallFetchTimeout bounds the delta index and delta-file fetches, which
// are small. The multi-gigabyte dump is streamed with no overall timeout so a
// slow body is not cut off.
const offDumpSmallFetchTimeout = 2 * time.Minute

// OffDumpController imports the OpenFoodFacts bulk dump (and its daily deltas)
// into a dedicated store so a barcode resolves offline without a live API call.
// It backs OfflineProductLookup and runs on a schedule like the other
// controllers' Dispatch goroutines.
type OffDumpController struct {
	Logger        *zerolog.Logger
	Configuration *configuration.OpenFoodFactsConfiguration
	Repository    *dbController.OffDumpRepository

	// HTTPClient, DeltaIndexURL and DeltaBaseURL are overridable for tests and
	// self-hosted mirrors. DeltaIndexURL and DeltaBaseURL default to the public
	// OFF endpoints in NewOffDumpController.
	HTTPClient    *http.Client
	DeltaIndexURL string
	DeltaBaseURL  string

	// smallFetchTimeout bounds the delta index and delta fetches; zero falls
	// back to offDumpSmallFetchTimeout.
	smallFetchTimeout time.Duration

	ctx          context.Context
	cancel       context.CancelFunc
	dispatchOnce sync.Once
	workers      sync.WaitGroup
	mu           sync.Mutex
	running      bool
	stopped      bool

	// deltaFailures counts consecutive unreadable imports per delta file,
	// guarded by mu, so a stuck delta is retried a bounded number of times.
	deltaFailures map[string]int
}

// NewOffDumpController constructs an OffDumpController backed by an opened dump
// repository.
func NewOffDumpController(
	logger *zerolog.Logger,
	config *configuration.OpenFoodFactsConfiguration,
	repository *dbController.OffDumpRepository,
) *OffDumpController {
	ctx, cancel := context.WithCancel(context.Background())
	return &OffDumpController{
		Logger:            logger,
		Configuration:     config,
		Repository:        repository,
		DeltaIndexURL:     util.OpenFoodFactsDeltaIndexURL,
		DeltaBaseURL:      util.OpenFoodFactsDeltaBaseURL,
		smallFetchTimeout: offDumpSmallFetchTimeout,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Dispatch starts the import goroutine when the feature is enabled: an initial
// run, then a periodic check for expiry or new deltas.
func (c *OffDumpController) Dispatch() {
	if c.Configuration == nil || !c.Configuration.DumpImportEnabled || c.Repository == nil {
		c.logInfo("OpenFoodFacts dump import is disabled")
		return
	}
	c.dispatchOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.stopped || c.ctx.Err() != nil {
			return
		}
		c.workers.Add(1)
		go func() {
			defer c.workers.Done()
			c.logInfo("OpenFoodFacts dump import dispatched (expiry: %d days)", c.Configuration.DumpImportExpiryDays)
			c.runGuarded()
			interval := time.Duration(util.OpenFoodFactsDumpCheckIntervalHours) * time.Hour
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-c.ctx.Done():
					return
				case <-ticker.C:
					c.runGuarded()
				}
			}
		}()
	})
}

// Stop ends the dispatch goroutine and waits briefly for an in-flight import.
func (c *OffDumpController) Stop() {
	c.mu.Lock()
	c.stopped = true
	c.cancel()
	c.mu.Unlock()

	done := make(chan struct{})
	go func() {
		c.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(util.OpenFoodFactsDumpStopWaitSeconds * time.Second):
		c.logWarn("OpenFoodFacts dump controller stop timed out waiting for worker")
	}
}

// runGuarded runs one import unless another is already in flight.
func (c *OffDumpController) runGuarded() {
	c.mu.Lock()
	if c.running || c.stopped {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.running = false
		c.mu.Unlock()
	}()

	runCtx, cancel := context.WithCancel(c.ctx)
	defer cancel()
	if err := c.RunOnce(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		c.logWarn("OpenFoodFacts dump import run completed with errors: %s", err)
	}
}

// RunOnce performs a single import pass: a full dump when it is due, otherwise
// the deltas published since the last import. A no-op when disabled.
func (c *OffDumpController) RunOnce(ctx context.Context) error {
	if c.Configuration == nil || !c.Configuration.DumpImportEnabled || c.Repository == nil {
		return nil
	}
	due, err := c.fullImportDue()
	if err != nil {
		return err
	}
	if due {
		return c.importFullDump(ctx)
	}
	return c.importDeltas(ctx)
}

// LookupProduct resolves a barcode from the imported dump. It reports false on
// a miss so the caller falls back to the live API, and swallows read errors so
// a broken dump never fails a scan.
func (c *OffDumpController) LookupProduct(barcode string) (database.Product, bool) {
	if c.Repository == nil || barcode == "" {
		return database.Product{}, false
	}
	raw, found, err := c.Repository.Lookup(barcode)
	if err != nil {
		c.logWarn("OpenFoodFacts dump lookup for barcode '%s' failed: %s", barcode, err)
		return database.Product{}, false
	}
	if !found {
		return database.Product{}, false
	}
	var product external.OpenFoodFactsDumpProduct
	if err := json.Unmarshal([]byte(raw), &product); err != nil {
		c.logWarn("OpenFoodFacts dump entry for barcode '%s' is unreadable: %s", barcode, err)
		return database.Product{}, false
	}
	return offDumpProductToProduct(&product), true
}

func (c *OffDumpController) fullImportDue() (bool, error) {
	value, err := c.Repository.GetState(util.OffDumpStateLastFullImport)
	if err != nil {
		return false, err
	}
	if value == "" {
		return true, nil
	}
	importedAt, parseErr := time.Parse(time.RFC3339, value)
	if parseErr != nil {
		return true, nil
	}
	expiry := time.Duration(c.Configuration.DumpImportExpiryDays) * 24 * time.Hour
	return time.Since(importedAt) >= expiry, nil
}

// importFullDump stages the full dump to disk and imports it in two phases, so
// an interruption resumes instead of restarting. The download is staged to a
// .part file and resumed by HTTP range; a completed download is renamed to
// .ready and imported, and the import tracks how many records it consumed.
func (c *OffDumpController) importFullDump(ctx context.Context) error {
	readyPath, err := c.downloadFullDump(ctx)
	if err != nil {
		return err
	}
	skip, err := c.stagedImportRecords()
	if err != nil {
		return err
	}
	count, err := c.importStagedDump(ctx, readyPath, skip)
	if err != nil {
		c.discardUnreadableStagedDump(readyPath, err)
		return err
	}

	importedAt := time.Now().UTC()
	if err := c.Repository.SetState(util.OffDumpStateLastFullImport, importedAt.Format(time.RFC3339)); err != nil {
		return err
	}
	// Anchor the delta baseline to the dump's Last-Modified time so changes made
	// after the dump was generated are still applied, and deltas already covered
	// by the dump are not replayed over current data.
	baseline := importedAt.Unix()
	if lastModified, stateErr := c.Repository.GetState(util.OffDumpStateFullImportLastModified); stateErr == nil && lastModified != "" {
		if parsed, parseErr := http.ParseTime(lastModified); parseErr == nil {
			baseline = parsed.Unix()
		}
	}
	if err := c.Repository.SetState(util.OffDumpStateLastDeltaEnd, strconv.FormatInt(baseline, 10)); err != nil {
		return err
	}
	if err := c.clearFullImportState(); err != nil {
		return err
	}
	if removeErr := os.Remove(readyPath); removeErr != nil && !os.IsNotExist(removeErr) {
		c.logWarn("OpenFoodFacts dump: removing staged file failed: %s", removeErr)
	}
	c.logInfo("OpenFoodFacts dump import complete: %d products, delta baseline %d", count, baseline)
	return nil
}

// downloadFullDump returns the path to a fully staged dump, resuming a partial
// download by byte range and discarding it when the remote object changed.
func (c *OffDumpController) downloadFullDump(ctx context.Context) (string, error) {
	partPath, readyPath := c.stagingPaths()
	storedURL, err := c.Repository.GetState(util.OffDumpStateFullImportURL)
	if err != nil {
		return "", err
	}
	storedETag, err := c.Repository.GetState(util.OffDumpStateFullImportETag)
	if err != nil {
		return "", err
	}
	// A staged dump is only reused when it belongs to the configured URL and has
	// not reached the re-download cadence; otherwise an old object (a switched
	// mirror) or a stale one would be imported instead of a fresh download.
	if info, statErr := os.Stat(readyPath); statErr == nil {
		switch {
		case !c.stagedDumpBelongsTo(storedURL):
			reason := fmt.Sprintf("staged from %q, not %q", storedURL, c.dumpURL())
			if storedURL == "" {
				reason = "staged with no recorded origin and no import in progress"
			}
			c.discardStagedDownload(readyPath, reason)
			storedURL, storedETag = "", ""
		case c.stagedDumpExpired(info.ModTime()):
			c.discardStagedDownload(readyPath, "older than the configured expiry")
			storedURL, storedETag = "", ""
		default:
			c.logInfo("OpenFoodFacts dump: reusing staged download %s", readyPath)
			return readyPath, nil
		}
	}
	// The checkpoint counts records consumed from the staged dump. Once no usable
	// .ready is staged, it must be zero: resetting it here rather than after the
	// rename means a crash mid-download cannot leave a stale count that a later
	// .ready skips over, silently dropping that many products from the import.
	if err := c.Repository.SetState(util.OffDumpStateFullImportRecords, "0"); err != nil {
		return "", err
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(partPath), 0o750); mkdirErr != nil {
		return "", fmt.Errorf("%w: %s", proviantErrors.ErrOpenFoodFactsDumpStagingUnavailable, mkdirErr)
	}

	localSize, err := fileSize(partPath)
	if err != nil {
		return "", err
	}

	resumeFrom, validator := int64(0), ""
	if localSize > 0 && storedETag != "" && storedURL == c.dumpURL() {
		resumeFrom, validator = localSize, storedETag
		c.logInfo("OpenFoodFacts dump: resuming download at %d MB", localSize>>20)
	} else {
		c.logInfo("OpenFoodFacts dump: downloading %s", c.dumpURL())
	}

	resp, appendMode, err := c.openDumpStream(ctx, resumeFrom, validator)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logWarn("OpenFoodFacts dump: closing download body failed: %s", closeErr)
		}
	}()

	if err := c.Repository.SetState(util.OffDumpStateFullImportURL, c.dumpURL()); err != nil {
		return "", err
	}
	if err := c.Repository.SetState(util.OffDumpStateFullImportETag, resp.Header.Get("ETag")); err != nil {
		return "", err
	}
	if err := c.Repository.SetState(util.OffDumpStateFullImportLastModified, resp.Header.Get("Last-Modified")); err != nil {
		return "", err
	}

	flags := os.O_CREATE | os.O_WRONLY
	resumeOffset := int64(0)
	if appendMode {
		flags |= os.O_APPEND
		resumeOffset = resumeFrom
	} else {
		flags |= os.O_TRUNC
	}
	staging, err := os.OpenFile(partPath, flags, 0o600) // #nosec G304 - path is the configured staging directory plus the dump's derived file name
	if err != nil {
		return "", fmt.Errorf("%w: %s", proviantErrors.ErrOpenFoodFactsDumpStagingUnavailable, err)
	}
	expected := expectedDumpSize(resp, resumeOffset)
	written, copyErr := io.Copy(c.dumpProgressWriter(staging, resumeOffset, expected), resp.Body)
	if copyErr != nil {
		c.closeStagingFile(staging)
		return "", fmt.Errorf("writing dump to staging: %w", copyErr)
	}
	total := written + resumeOffset
	if expected >= 0 && total != expected {
		c.closeStagingFile(staging)
		return "", fmt.Errorf("dump download truncated: wrote %d of %d bytes", total, expected)
	}
	if syncErr := staging.Sync(); syncErr != nil {
		c.closeStagingFile(staging)
		return "", syncErr
	}
	if closeErr := staging.Close(); closeErr != nil {
		return "", closeErr
	}
	if err := os.Rename(partPath, readyPath); err != nil {
		return "", err
	}
	c.logInfo("OpenFoodFacts dump: staged download complete at %s", readyPath)
	return readyPath, nil
}

// dumpProgressWriter wraps the staging file so a multi-gigabyte download reports
// progress instead of looking stalled until it finishes.
func (c *OffDumpController) dumpProgressWriter(staging io.Writer, offset, expected int64) io.Writer {
	return &offDumpProgressWriter{
		writer:     staging,
		nextReport: offDumpProgressInterval,
		report: func(written int64) {
			downloaded := offset + written
			if expected > 0 {
				c.logInfo("OpenFoodFacts dump: downloaded %d MB of %d MB (%d%%)", downloaded>>20, expected>>20, downloaded*100/expected)
				return
			}
			c.logInfo("OpenFoodFacts dump: downloaded %d MB", downloaded>>20)
		},
	}
}

// offDumpProgressWriter counts copied bytes and reports at a fixed interval.
type offDumpProgressWriter struct {
	writer     io.Writer
	written    int64
	nextReport int64
	report     func(written int64)
}

// Write copies data to the underlying writer and reports at each interval.
func (w *offDumpProgressWriter) Write(data []byte) (int, error) {
	written, err := w.writer.Write(data)
	w.written += int64(written)
	if w.written >= w.nextReport {
		w.report(w.written)
		w.nextReport = w.written + offDumpProgressInterval
	}
	return written, err
}

// openDumpStream issues the dump request, resuming from resumeFrom with the
// given validator when both are set. It retries once from scratch when the
// server rejects the range or the object changed, so a stale partial download
// is never stitched onto a different dump.
func (c *OffDumpController) openDumpStream(ctx context.Context, resumeFrom int64, validator string) (*http.Response, bool, error) {
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := c.fetchDump(ctx, resumeFrom, validator)
		if err != nil {
			return nil, false, fmt.Errorf("downloading dump: %w", err)
		}
		if resumeFrom > 0 && resp.StatusCode == http.StatusPartialContent {
			if respETag := resp.Header.Get("ETag"); respETag != "" && respETag != validator {
				c.closeBody(resp)
				resumeFrom, validator = 0, ""
				continue
			}
			return resp, true, nil
		}
		if resumeFrom > 0 && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			c.closeBody(resp)
			if removeErr := os.Remove(c.partPath()); removeErr != nil && !os.IsNotExist(removeErr) {
				return nil, false, removeErr
			}
			resumeFrom, validator = 0, ""
			continue
		}
		if resp.StatusCode != http.StatusOK {
			c.closeBody(resp)
			return nil, false, fmt.Errorf("dump download returned status %d", resp.StatusCode)
		}
		return resp, false, nil
	}
	return nil, false, fmt.Errorf("dump download could not resume after validation retry")
}

func (c *OffDumpController) fetchDump(ctx context.Context, resumeFrom int64, validator string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.dumpURL(), nil)
	if err != nil {
		return nil, err
	}
	if resumeFrom > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(resumeFrom, 10)+"-")
		if validator != "" {
			req.Header.Set("If-Range", validator)
		}
	}
	return c.httpClient().Do(req)
}

// importStagedDump imports a fully staged dump, skipping skip already-consumed
// records and persisting the consumed count after each batch.
func (c *OffDumpController) importStagedDump(ctx context.Context, path string, skip int) (int, error) {
	staging, err := os.Open(path) // #nosec G304 - path is this controller's own staged dump file
	if err != nil {
		return 0, fmt.Errorf("%w: %s", proviantErrors.ErrOpenFoodFactsDumpStagingUnavailable, err)
	}
	defer func() {
		if closeErr := staging.Close(); closeErr != nil {
			c.logWarn("OpenFoodFacts dump: closing staged file failed: %s", closeErr)
		}
	}()
	if skip > 0 {
		c.logInfo("OpenFoodFacts dump: resuming import of %s after %d records", path, skip)
	} else {
		c.logInfo("OpenFoodFacts dump: importing staged dump %s", path)
	}
	imported, _, err := c.importJSONLStream(ctx, staging, skip, c.persistStagedRecords)
	return imported, err
}

func (c *OffDumpController) persistStagedRecords(consumed int) error {
	return c.Repository.SetState(util.OffDumpStateFullImportRecords, strconv.Itoa(consumed))
}

// discardUnreadableStagedDump removes a staged dump that cannot be decoded and
// clears its full import state — URL, validators, and checkpoint — so the next
// run re-downloads instead of re-importing the same broken file forever. A
// recoverable error (a shutdown or a database write) keeps the file, since the
// import resumes from it.
func (c *OffDumpController) discardUnreadableStagedDump(path string, importErr error) {
	if !errors.Is(importErr, errDumpStreamUnreadable) {
		return
	}
	c.discardStagedDownload(path, "unreadable: "+importErr.Error())
}

// discardStagedDownload drops a staged full dump that must not be imported and
// clears its state, so the next download starts from scratch instead of reusing
// it.
func (c *OffDumpController) discardStagedDownload(path, reason string) {
	c.logWarn("OpenFoodFacts dump: discarding staged download %s (%s)", path, reason)
	if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
		c.logWarn("OpenFoodFacts dump: removing staged file failed: %s", removeErr)
	}
	if stateErr := c.clearFullImportState(); stateErr != nil {
		c.logWarn("OpenFoodFacts dump: clearing staged download state failed: %s", stateErr)
	}
}

// stagedDumpExpired reports whether a staged download has reached the full
// re-download cadence, so a stale stage never wins over a fresh dump.
func (c *OffDumpController) stagedDumpExpired(modified time.Time) bool {
	if c.Configuration == nil || c.Configuration.DumpImportExpiryDays <= 0 {
		return false
	}
	return time.Since(modified) >= time.Duration(c.Configuration.DumpImportExpiryDays)*24*time.Hour
}

// stagedDumpBelongsTo reports whether a staged full dump belongs to the
// configured dump URL. An unknown origin is trusted only when an import is
// already in progress, since a download records the URL before the staged file
// appears; anything else is discarded and re-downloaded.
func (c *OffDumpController) stagedDumpBelongsTo(storedURL string) bool {
	if storedURL == "" {
		records, err := c.stagedImportRecords()
		return err == nil && records > 0
	}
	return storedURL == c.dumpURL()
}

func (c *OffDumpController) stagedImportRecords() (int, error) {
	value, err := c.Repository.GetState(util.OffDumpStateFullImportRecords)
	if err != nil {
		return 0, err
	}
	if value == "" {
		return 0, nil
	}
	records, parseErr := strconv.Atoi(value)
	if parseErr != nil {
		c.logWarn("OpenFoodFacts dump: unreadable import checkpoint %q, restarting the import: %s", value, parseErr)
		return 0, nil
	}
	return records, nil
}

func (c *OffDumpController) clearFullImportState() error {
	keys := []string{
		util.OffDumpStateFullImportURL,
		util.OffDumpStateFullImportETag,
		util.OffDumpStateFullImportLastModified,
		util.OffDumpStateFullImportRecords,
	}
	for _, key := range keys {
		if err := c.Repository.SetState(key, ""); err != nil {
			return err
		}
	}
	return nil
}

func (c *OffDumpController) stagingPaths() (partPath, readyPath string) {
	base := filepath.Join(c.stagingDir(), util.OffDumpStagingFilename(c.dumpDatabaseName()))
	return base + util.OffDumpStagingPartSuffix, base + util.OffDumpStagingReadySuffix
}

func (c *OffDumpController) partPath() string {
	partPath, _ := c.stagingPaths()
	return partPath
}

func (c *OffDumpController) stagingDir() string {
	if c.Configuration != nil && c.Configuration.DumpStagingPath != "" {
		return c.Configuration.DumpStagingPath
	}
	return util.OffDumpStagingDir
}

func (c *OffDumpController) dumpDatabaseName() string {
	if c.Configuration == nil {
		return ""
	}
	return c.Configuration.DumpDatabase
}

func (c *OffDumpController) closeStagingFile(file *os.File) {
	if closeErr := file.Close(); closeErr != nil {
		c.logWarn("OpenFoodFacts dump: closing staging file failed: %s", closeErr)
	}
}

func (c *OffDumpController) closeBody(resp *http.Response) {
	if closeErr := resp.Body.Close(); closeErr != nil {
		c.logWarn("OpenFoodFacts dump: closing response body failed: %s", closeErr)
	}
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return info.Size(), nil
}

// expectedDumpSize returns the total dump size the response should yield, or -1
// when the server did not advertise it (for example a chunked body).
func expectedDumpSize(resp *http.Response, resumeFrom int64) int64 {
	if resp.StatusCode == http.StatusPartialContent {
		if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
			if idx := strings.LastIndex(contentRange, "/"); idx >= 0 {
				if total, parseErr := strconv.ParseInt(contentRange[idx+1:], 10, 64); parseErr == nil {
					return total
				}
			}
		}
		if resp.ContentLength >= 0 {
			return resumeFrom + resp.ContentLength
		}
		return -1
	}
	return resp.ContentLength
}

func (c *OffDumpController) importDeltas(ctx context.Context) error {
	lastEnd, err := c.lastDeltaEnd()
	if err != nil {
		return err
	}
	deltas, err := c.fetchDeltaIndex(ctx)
	if err != nil {
		return err
	}

	windowDays := c.Configuration.DumpDeltaWindowDays
	var cutoff int64
	if windowDays > 0 {
		cutoff = time.Now().Add(-time.Duration(windowDays) * 24 * time.Hour).Unix()
	}

	applied := 0
	for i := range deltas {
		if err := ctx.Err(); err != nil {
			return err
		}
		delta := deltas[i]
		if delta.endAt <= lastEnd {
			continue
		}
		if windowDays > 0 && delta.endAt < cutoff {
			continue
		}
		inserted, skipped, err := c.importDeltaFile(ctx, delta)
		if err != nil {
			return err
		}
		// A delta whose every record was unreadable means the file format is
		// unexpected; hold the baseline and retry, but only up to maxDeltaAttempts
		// so one permanently bad file cannot stall every newer delta.
		if inserted == 0 && skipped > 0 {
			attempts := c.deltaAttempts(delta.name)
			if attempts < maxDeltaAttempts {
				return fmt.Errorf("delta %s: all %d records were unreadable (attempt %d/%d), holding the delta baseline", delta.name, skipped, attempts, maxDeltaAttempts)
			}
			c.logWarn("OpenFoodFacts dump: delta %s unreadable after %d attempts, skipping it", delta.name, attempts)
		} else {
			c.clearDeltaFailures(delta.name)
		}
		if err := c.Repository.SetState(util.OffDumpStateLastDeltaEnd, strconv.FormatInt(delta.endAt, 10)); err != nil {
			return err
		}
		lastEnd = delta.endAt
		applied++
	}
	if applied > 0 {
		c.logInfo("OpenFoodFacts dump applied %d delta file(s)", applied)
	}
	return nil
}

func (c *OffDumpController) importDeltaFile(ctx context.Context, delta deltaFile) (int, int, error) {
	c.logInfo("OpenFoodFacts dump applying delta %s", delta.name)
	resp, err := c.doGet(ctx, c.DeltaBaseURL+delta.name)
	if err != nil {
		return 0, 0, fmt.Errorf("downloading delta %s: %w", delta.name, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logWarn("OpenFoodFacts dump: closing delta body failed: %s", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("delta %s returned status %d", delta.name, resp.StatusCode)
	}
	return c.importJSONLStream(ctx, resp.Body, 0, nil)
}

// deltaAttempts records another failed import of deltaName and returns the
// running count for that file.
func (c *OffDumpController) deltaAttempts(deltaName string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deltaFailures == nil {
		c.deltaFailures = make(map[string]int)
	}
	c.deltaFailures[deltaName]++
	return c.deltaFailures[deltaName]
}

// clearDeltaFailures forgets the failure history of a delta that imported cleanly.
func (c *OffDumpController) clearDeltaFailures(deltaName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.deltaFailures, deltaName)
}

// importJSONLStream decodes a gzip-compressed stream of JSON product objects,
// filters each to the fields Proviant uses, and upserts them in batches. It
// never buffers the whole file. A line that is not a readable product — invalid
// JSON or an unexpected field type — is skipped rather than aborting the import,
// so one malformed product cannot stall a multi-million-row dump. It skips the
// first skip lines so an interrupted import resumes, and reports each batch's
// consumed count through checkpoint when one is set. It returns the number of
// products upserted and the number of lines skipped as unreadable.
func (c *OffDumpController) importJSONLStream(ctx context.Context, reader io.Reader, skip int, checkpoint func(consumed int) error) (int, int, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: opening gzip stream: %w", errDumpStreamUnreadable, err)
	}
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil {
			c.logWarn("OpenFoodFacts dump: closing gzip stream failed: %s", closeErr)
		}
	}()

	lineReader := bufio.NewReaderSize(gzipReader, offDumpReadBufferSize)
	batch := make([]database.OffProduct, 0, util.OpenFoodFactsDumpBatchSize)
	inserted := 0
	consumed := 0
	skipped := 0

	for {
		if err := ctx.Err(); err != nil {
			return inserted, skipped, err
		}
		line, readErr := lineReader.ReadBytes('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return inserted, skipped, fmt.Errorf("%w: reading dump stream: %w", errDumpStreamUnreadable, readErr)
		}
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			if errors.Is(readErr, io.EOF) {
				break
			}
			continue
		}
		consumed++
		if consumed <= skip {
			continue
		}
		var raw external.OpenFoodFactsDumpProduct
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			skipped++
			continue
		}
		barcode := strings.TrimSpace(string(raw.Code))
		if barcode == "" {
			continue
		}
		raw.Code = external.FlexibleString(barcode)
		filtered, marshalErr := json.Marshal(raw)
		if marshalErr != nil {
			skipped++
			continue
		}
		batch = append(batch, database.OffProduct{Barcode: barcode, Data: string(filtered)})
		if len(batch) < util.OpenFoodFactsDumpBatchSize {
			continue
		}
		if err := c.Repository.UpsertBatch(ctx, batch); err != nil {
			return inserted, skipped, err
		}
		inserted += len(batch)
		batch = batch[:0]
		if checkpoint != nil {
			if err := checkpoint(consumed); err != nil {
				return inserted, skipped, err
			}
		}
		if inserted%100000 == 0 {
			c.logInfo("OpenFoodFacts dump import progress: %d products", inserted)
		}
	}

	if len(batch) > 0 {
		if err := c.Repository.UpsertBatch(ctx, batch); err != nil {
			return inserted, skipped, err
		}
		inserted += len(batch)
	}
	if checkpoint != nil {
		if err := checkpoint(consumed); err != nil {
			return inserted, skipped, err
		}
	}
	if skipped > 0 {
		c.logWarn("OpenFoodFacts dump: skipped %d unreadable products", skipped)
	}
	return inserted, skipped, nil
}

func (c *OffDumpController) lastDeltaEnd() (int64, error) {
	value, err := c.Repository.GetState(util.OffDumpStateLastDeltaEnd)
	if err != nil {
		return 0, err
	}
	if value == "" {
		return 0, nil
	}
	parsed, parseErr := strconv.ParseInt(value, 10, 64)
	if parseErr != nil {
		c.logWarn("OpenFoodFacts dump: unreadable delta baseline %q, replaying from the start: %s", value, parseErr)
		return 0, nil
	}
	return parsed, nil
}

// deltaFile is one entry of the OFF delta index. The filename encodes the UNIX
// timestamps of the first and last change it contains, so entries can be
// applied in chronological (alphabetical) order.
type deltaFile struct {
	name    string
	startAt int64
	endAt   int64
}

func (c *OffDumpController) fetchDeltaIndex(ctx context.Context) ([]deltaFile, error) {
	resp, err := c.doGet(ctx, c.DeltaIndexURL)
	if err != nil {
		return nil, fmt.Errorf("fetching delta index: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logWarn("OpenFoodFacts dump: closing delta index body failed: %s", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("delta index returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var deltas []deltaFile
	for scanner.Scan() {
		delta, ok := parseDeltaFilename(strings.TrimSpace(scanner.Text()))
		if !ok {
			continue
		}
		deltas = append(deltas, delta)
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, fmt.Errorf("reading delta index: %w", scanErr)
	}
	sort.Slice(deltas, func(i, j int) bool { return deltas[i].endAt < deltas[j].endAt })
	return deltas, nil
}

// parseDeltaFilename parses "openfoodfacts_products_<start>_<end>.json.gz".
func parseDeltaFilename(name string) (deltaFile, bool) {
	const prefix = "openfoodfacts_products_"
	const suffix = ".json.gz"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return deltaFile{}, false
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
	parts := strings.Split(trimmed, "_")
	if len(parts) != 2 {
		return deltaFile{}, false
	}
	startAt, startErr := strconv.ParseInt(parts[0], 10, 64)
	endAt, endErr := strconv.ParseInt(parts[1], 10, 64)
	if startErr != nil || endErr != nil {
		return deltaFile{}, false
	}
	return deltaFile{name: name, startAt: startAt, endAt: endAt}, true
}

func (c *OffDumpController) dumpURL() string {
	if c.Configuration != nil && c.Configuration.DumpURL != "" {
		return c.Configuration.DumpURL
	}
	return util.OpenFoodFactsDumpURL
}

func (c *OffDumpController) doGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.smallFetchClient().Do(req)
}

func (c *OffDumpController) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{}
}

// smallFetchClient bounds the small delta fetches with an overall timeout, so a
// stalled connection cannot hang a run indefinitely. A caller-provided
// HTTPClient is used as-is.
func (c *OffDumpController) smallFetchClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	timeout := c.smallFetchTimeout
	if timeout <= 0 {
		timeout = offDumpSmallFetchTimeout
	}
	return &http.Client{Timeout: timeout}
}

func offDumpProductToProduct(product *external.OpenFoodFactsDumpProduct) database.Product {
	var co2 *float64
	if product.EcoscoreData.Agribalyse.CO2Total > 0 {
		value := float64(product.EcoscoreData.Agribalyse.CO2Total)
		co2 = &value
	}
	return database.Product{
		Barcode:     string(product.Code),
		ProductName: product.ProductName,
		Categories:  product.Categories,
		Countries:   product.Countries,
		ImageURL:    product.ImageURL,
		CO2KgPerKg:  co2,
		StorageHint: product.ConservationConditions,
	}
}

func (c *OffDumpController) logInfo(format string, args ...any) {
	if c.Logger != nil {
		c.Logger.Info().Msgf(format, args...)
	}
}

func (c *OffDumpController) logWarn(format string, args ...any) {
	if c.Logger != nil {
		c.Logger.Warn().Msgf(format, args...)
	}
}
