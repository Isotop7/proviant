package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// BackupController writes one versioned household backup document per
// household to the configured directory on a fixed interval.
type BackupController struct {
	Logger        *zerolog.Logger
	Configuration *configuration.BackupConfiguration
	DB            *gorm.DB
	ctx           context.Context
	cancel        context.CancelFunc
	dispatchOnce  sync.Once
	workers       sync.WaitGroup
	mu            sync.Mutex
	stopped       bool
}

// NewBackupController constructs a BackupController.
func NewBackupController(
	logger *zerolog.Logger,
	config *configuration.BackupConfiguration,
	db *gorm.DB,
) *BackupController {
	ctx, cancel := context.WithCancel(context.Background())
	return &BackupController{
		Logger:        logger,
		Configuration: config,
		DB:            db,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Stop ends the dispatch goroutine and waits briefly for it to exit.
func (c *BackupController) Stop() {
	c.mu.Lock()
	c.stopped = true
	c.cancel()
	c.mu.Unlock()

	done := make(chan struct{})
	timedOut := make(chan struct{})
	go func() {
		c.workers.Wait()
		select {
		case <-timedOut:
			if c.Logger != nil {
				c.Logger.Warn().Msg("BackupController worker completed after stop timeout")
			}
		default:
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(util.DefaultBackupStopWaitSeconds * time.Second):
		close(timedOut)
		if c.Logger != nil {
			c.Logger.Warn().Msg("BackupController stop timed out waiting for worker")
		}
	}
}

// RunOnce writes one backup per household and prunes expired files. A failure
// for one household is logged and does not stop the others: a single malformed
// row must not silently disable the whole backup job.
func (c *BackupController) RunOnce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.Configuration == nil || !c.Configuration.Enabled || c.DB == nil {
		return nil
	}
	directory := strings.TrimSpace(c.Configuration.Directory)
	if directory == "" {
		return fmt.Errorf("%w: empty directory", errors.ErrBackupInvalidDirectory)
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return fmt.Errorf("%w: %s", errors.ErrBackupInvalidDirectory, err)
	}

	var households []dbModel.Household
	if err := c.DB.WithContext(ctx).Find(&households).Error; err != nil {
		return err
	}

	var firstErr error
	for i := range households {
		if err := ctx.Err(); err != nil {
			return err
		}
		household := &households[i]
		path, err := c.writeHouseholdBackup(ctx, directory, household)
		if err != nil {
			if c.Logger != nil {
				c.Logger.Error().Msgf("Backup job: household %d failed: %s", household.ID, err)
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if c.Logger != nil {
			c.Logger.Info().Msgf("Backup job: household %d written to %s", household.ID, path)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		c.pruneHouseholdBackups(directory, household.ID)
	}
	return firstErr
}

// writeHouseholdBackup builds and atomically writes one household document.
// The temp-file-plus-rename keeps a crash from leaving a truncated backup that
// looks like a valid file to the restore endpoint.
func (c *BackupController) writeHouseholdBackup(
	ctx context.Context,
	directory string,
	household *dbModel.Household,
) (string, error) {
	document, err := BuildFullExport(c.DB.WithContext(ctx), c.Logger, household.ID, household.AdminID, true)
	if err != nil {
		return "", err
	}
	payload, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("%shousehold-%d-%s.json", util.BackupFilePrefix, household.ID, time.Now().UTC().Format("20060102T150405.000Z"))
	path := filepath.Join(directory, filename)
	tempPath := path + ".tmp"
	if err := writeBackupFile(ctx, tempPath, payload); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := ctx.Err(); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	return path, nil
}

// backupWriteChunkSize bounds one file.Write call so cancellation is observed
// between chunks instead of only after the whole document is written.
const backupWriteChunkSize = 1 << 20

// writeBackupFile writes a backup document with cancellation checks between
// chunks. os.WriteFile cannot be interrupted by Stop(), so a shutdown could
// otherwise wait behind a full file write.
func writeBackupFile(ctx context.Context, path string, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 - path is generated under the configured backup directory
	if err != nil {
		return err
	}
	for offset := 0; offset < len(payload); {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return err
		}
		end := offset + backupWriteChunkSize
		if end > len(payload) {
			end = len(payload)
		}
		if _, err := file.Write(payload[offset:end]); err != nil {
			_ = file.Close()
			return err
		}
		offset = end
	}
	return file.Close()
}

// pruneHouseholdBackups deletes the oldest documents for one household beyond
// the configured retention. Retention 0 keeps every file. Files are matched by
// the household-specific filename prefix so one household's prune can never
// touch another's backups.
func (c *BackupController) pruneHouseholdBackups(directory string, householdID uint) {
	retention := c.Configuration.Retention
	if retention <= 0 {
		return
	}
	prefix := fmt.Sprintf("%shousehold-%d-", util.BackupFilePrefix, householdID)
	entries, err := os.ReadDir(directory)
	if err != nil {
		if c.Logger != nil {
			c.Logger.Warn().Msgf("Backup job: cannot read %s for retention: %s", directory, err)
		}
		return
	}

	names := make([]string, 0, len(entries))
	for i := range entries {
		if entries[i].IsDir() || !strings.HasPrefix(entries[i].Name(), prefix) || !strings.HasSuffix(entries[i].Name(), ".json") {
			continue
		}
		names = append(names, entries[i].Name())
	}
	// The timestamp in the filename is UTC and sorts lexicographically, so a
	// reverse sort leaves the newest files at the front.
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if retention >= len(names) {
		return
	}
	for _, name := range names[retention:] {
		if err := os.Remove(filepath.Join(directory, name)); err != nil && c.Logger != nil {
			c.Logger.Warn().Msgf("Backup job: failed to prune %s: %s", name, err)
		}
	}
}

// Dispatch starts the periodic backup goroutine when enabled in configuration.
func (c *BackupController) Dispatch() {
	if c.Configuration == nil || !c.Configuration.Enabled {
		if c.Logger != nil {
			c.Logger.Info().Msg("BackupController is disabled")
		}
		return
	}
	intervalHours := c.Configuration.IntervalHours
	if intervalHours <= 0 {
		intervalHours = util.DefaultBackupIntervalHours
	}
	interval := time.Duration(intervalHours) * time.Hour
	c.dispatchOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.stopped || c.ctx.Err() != nil {
			return
		}
		c.workers.Add(1)
		go func() {
			defer c.workers.Done()
			if c.Logger != nil {
				c.Logger.Info().Msgf("BackupController dispatched (interval: %d hours, directory: %s)", intervalHours, c.Configuration.Directory)
			}
			runCtx, runCancel := context.WithTimeout(c.ctx, 10*time.Minute)
			if err := c.RunOnce(runCtx); err != nil && c.Logger != nil {
				c.Logger.Warn().Msgf("BackupController initial run completed with errors: %s", err)
			}
			runCancel()

			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-c.ctx.Done():
					return
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(c.ctx, 10*time.Minute)
					if err := c.RunOnce(ctx); err != nil && c.Logger != nil {
						c.Logger.Warn().Msgf("BackupController run completed with errors: %s", err)
					}
					cancel()
				}
			}
		}()
	})
}
