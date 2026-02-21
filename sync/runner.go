package sync

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
)

// Runner manages scheduled and manual sync execution
type Runner struct {
	engine    *Engine
	cron      *cron.Cron
	syncMap   map[string]cron.EntryID
	mu        sync.RWMutex
	logger    *slog.Logger
	running   bool
}

// NewRunner creates a new sync runner
func NewRunner(engine *Engine, logger *slog.Logger) *Runner {
	return &Runner{
		engine:  engine,
		cron:    cron.New(),
		syncMap: make(map[string]cron.EntryID),
		logger:  logger,
	}
}

// Start starts the runner and schedules all enabled syncs
func (r *Runner) Start(cfg *config.Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("runner already started")
	}

	for _, syncCfg := range cfg.Syncs {
		if !syncCfg.Enabled {
			r.logger.Info("sync disabled, skipping",
				slog.String("sync_id", syncCfg.ID),
			)
			continue
		}

		if syncCfg.Schedule == "" {
			r.logger.Info("no schedule defined for sync, skipping",
				slog.String("sync_id", syncCfg.ID),
			)
			continue
		}

		// Create a closure to capture the sync config
		syncCfg := syncCfg
		job := func() {
			if err := r.engine.ExecuteSync(&syncCfg); err != nil {
				r.logger.Error("sync execution failed",
					slog.String("sync_id", syncCfg.ID),
					slog.String("error", err.Error()),
				)
			}
		}

		entryID, err := r.cron.AddFunc(syncCfg.Schedule, job)
		if err != nil {
			return fmt.Errorf("failed to schedule sync %s: %w", syncCfg.ID, err)
		}

		r.syncMap[syncCfg.ID] = entryID
		r.logger.Info("sync scheduled",
			slog.String("sync_id", syncCfg.ID),
			slog.String("schedule", syncCfg.Schedule),
		)
	}

	r.cron.Start()
	r.running = true
	r.logger.Info("sync runner started")

	return nil
}

// Stop stops the runner
func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	r.cron.Stop()
	r.running = false
	r.logger.Info("sync runner stopped")
}

// ExecuteSyncNow immediately executes a sync, bypassing schedule
func (r *Runner) ExecuteSyncNow(syncID string, cfg *config.Config) error {
	var syncCfg *config.SyncConfig
	for i := range cfg.Syncs {
		if cfg.Syncs[i].ID == syncID {
			syncCfg = &cfg.Syncs[i]
			break
		}
	}

	if syncCfg == nil {
		return fmt.Errorf("sync not found: %s", syncID)
	}

	return r.engine.ExecuteSync(syncCfg)
}

// GetSyncStatus returns the status of a sync
func (r *Runner) GetSyncStatus(syncID string, store *storage.Store) (map[string]interface{}, error) {
	runs, err := store.GetSyncRuns(syncID, 10)
	if err != nil {
		return nil, err
	}

	syncObjects, err := store.GetSyncObjectsBySync(syncID)
	if err != nil {
		return nil, err
	}

	var lastRun *storage.SyncRun
	if len(runs) > 0 {
		lastRun = runs[0]
	}

	successCount := 0
	for _, obj := range syncObjects {
		if obj.LastSyncStatus == "success" || obj.LastSyncStatus == "in_sync" {
			successCount++
		}
	}

	return map[string]interface{}{
		"sync_id":         syncID,
		"last_run":        lastRun,
		"total_objects":   len(syncObjects),
		"synced_objects":  successCount,
		"failed_objects":  len(syncObjects) - successCount,
		"recent_runs":     runs,
	}, nil
}

// IsRunning returns whether the runner is actively running
func (r *Runner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// GetEntries returns all scheduled sync entries
func (r *Runner) GetEntries() map[string]cron.Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make(map[string]cron.Entry)
	for syncID, entryID := range r.syncMap {
		entry := r.cron.Entry(entryID)
		if entry.ID == entryID {
			entries[syncID] = entry
		}
	}
	return entries
}
