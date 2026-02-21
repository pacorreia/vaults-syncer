package sync

import (
	"crypto/md5"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
	"github.com/pacorreia/vaults-syncer/vault"
)

// Engine handles syncing between vaults
type Engine struct {
	cfg      *config.Config
	store    *storage.Store
	backends map[string]vault.Backend
	logger   *slog.Logger
}

// NewEngine creates a new sync engine
func NewEngine(cfg *config.Config, store *storage.Store, logger *slog.Logger) (*Engine, error) {
	engine := &Engine{
		cfg:      cfg,
		store:    store,
		backends: make(map[string]vault.Backend),
		logger:   logger,
	}

	// Create backends for all vaults
	for _, vaultCfg := range cfg.Vaults {
		backend, err := vault.NewBackend(&vaultCfg)
		if err != nil {
			engine.logger.Warn("failed to create vault backend",
				slog.String("vault_id", vaultCfg.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		
		// Test connection
		if err := backend.TestConnection(); err != nil {
			engine.logger.Warn("failed to connect to vault", 
				slog.String("vault_id", vaultCfg.ID),
				slog.String("error", err.Error()),
			)
		}

		engine.backends[vaultCfg.ID] = backend
	}

	return engine, nil
}

// ExecuteSync runs a sync operation
func (e *Engine) ExecuteSync(syncCfg *config.SyncConfig) error {
	startTime := time.Now()
	
	e.logger.Info("starting sync", slog.String("sync_id", syncCfg.ID))

	sourceBackend, ok := e.backends[syncCfg.Source]
	if !ok {
		return fmt.Errorf("source vault backend not found: %s", syncCfg.Source)
	}

	// Get all secrets from source
	secrets, err := sourceBackend.ListSecrets()
	if err != nil {
		msg := fmt.Sprintf("failed to list secrets from source: %v", err)
		e.logger.Error(msg)
		e.store.RecordSyncRun(syncCfg.ID, "failed", 0, 0, int64(time.Since(startTime).Milliseconds()), msg)
		return err
	}

	e.logger.Debug("fetched secrets from source", 
		slog.String("sync_id", syncCfg.ID),
		slog.Int("count", len(secrets)),
	)

	successCount := 0
	failureCount := 0

	// Determine concurrency level
	workerCount := syncCfg.ConcurrentWorkers
	if workerCount <= 0 {
		workerCount = 1 // Sequential by default
	}

	// Unidirectional: source → targets
	if syncCfg.SyncType == "unidirectional" {
		syncCount := e.executeSyncUnidirectionalConcurrent(syncCfg, sourceBackend, workerCount)
		successCount = syncCount.Success
		failureCount = syncCount.Failure
	} else if syncCfg.SyncType == "bidirectional" {
		// Bidirectional: source ↔ target (only 1:1 allowed)
		targetID := syncCfg.Targets[0]
		targetBackend, ok := e.backends[targetID]
		if !ok {
			return fmt.Errorf("target vault backend not found: %s", targetID)
		}
		syncCount := e.executeSyncBidirectionalConcurrent(syncCfg, sourceBackend, targetBackend, workerCount)
		successCount = syncCount.Success
		failureCount = syncCount.Failure
	}

	duration := time.Since(startTime).Milliseconds()
	status := "success"
	if failureCount > 0 {
		status = "partial"
	}
	if successCount == 0 {
		status = "failed"
	}

	e.store.RecordSyncRun(syncCfg.ID, status, successCount, failureCount, duration, "")
	e.logger.Info("sync completed",
		slog.String("sync_id", syncCfg.ID),
		slog.Int("succeeded", successCount),
		slog.Int("failed", failureCount),
		slog.Int64("duration_ms", duration),
	)

	return nil
}

// syncSecretUnidirectional syncs a secret in one direction with retries
func (e *Engine) syncSecretUnidirectional(syncID, sourceID, targetID, secretName string, sourceBackend vault.Backend) error {
	targetBackend, ok := e.backends[targetID]
	if !ok {
		return fmt.Errorf("target vault backend not found: %s", targetID)
	}

	// Get secret from source
	secret, err := sourceBackend.GetSecret(secretName)
	if err != nil {
		return fmt.Errorf("failed to get secret from source: %w", err)
	}

	// Apply transforms
	value := secret.Value
	// Transforms can be added here based on syncCfg.Transforms

	// Try to set in target with retries
	retryPolicy := config.RetryPolicy{
		MaxRetries:     3,
		InitialBackoff: 1000,
		MaxBackoff:     60000,
		Multiplier:     2.0,
	}

	err = withRetry(retryPolicy, func() error {
		return targetBackend.SetSecret(secretName, value)
	})

	if err != nil {
		// Record failure
		sourceChecksum := hashString(secret.Value)
		obj := &config.SyncObject{
			SyncID:       syncID,
			SourceVaultID: sourceID,
			TargetVaultID: targetID,
			SecretName:   secretName,
			SourceChecksum: sourceChecksum,
			LastSyncTime: time.Now().Unix(),
			LastSyncStatus: "failed",
			LastSyncError: err.Error(),
			DirectionLast: "source_to_target",
		}
		e.store.UpsertSyncObject(obj)
		return err
	}

	// Record success
	sourceChecksum := hashString(secret.Value)
	obj := &config.SyncObject{
		SyncID:        syncID,
		SourceVaultID: sourceID,
		TargetVaultID: targetID,
		SecretName:    secretName,
		SourceChecksum: sourceChecksum,
		TargetChecksum: sourceChecksum,
		LastSyncTime:  time.Now().Unix(),
		LastSyncStatus: "success",
		DirectionLast: "source_to_target",
	}
	e.store.UpsertSyncObject(obj)

	return nil
}

// syncSecretBidirectional syncs a secret bidirectionally
func (e *Engine) syncSecretBidirectional(syncID, sourceID, targetID, secretName string, sourceBackend, targetBackend vault.Backend) error {
	// Get from source
	sourceSecret, err := sourceBackend.GetSecret(secretName)
	if err != nil {
		return fmt.Errorf("failed to get secret from source: %w", err)
	}

	// Get from target
	targetSecret, err := targetBackend.GetSecret(secretName)
	if err != nil {
		// Secret doesn't exist in target yet, sync from source
		return e.syncSecretUnidirectional(syncID, sourceID, targetID, secretName, sourceBackend)
	}

	sourceChecksum := hashString(sourceSecret.Value)
	targetChecksum := hashString(targetSecret.Value)

	// If they're the same, no action needed
	if sourceChecksum == targetChecksum {
		obj := &config.SyncObject{
			SyncID:        syncID,
			SourceVaultID: sourceID,
			TargetVaultID: targetID,
			SecretName:    secretName,
			SourceChecksum: sourceChecksum,
			TargetChecksum: targetChecksum,
			LastSyncTime:  time.Now().Unix(),
			LastSyncStatus: "in_sync",
			DirectionLast: "none",
		}
		e.store.UpsertSyncObject(obj)
		return nil
	}

	// Get existing sync object to determine last direction
	existingObj, err := e.store.GetSyncObject(syncID, sourceID, targetID, secretName)
	if err != nil {
		return fmt.Errorf("failed to get sync object: %w", err)
	}

	// Determine which one to trust (most recently modified)
	var direction string
	if existingObj == nil {
		// First time seeing this divergence, sync from source to target
		direction = "source_to_target"
	} else {
		// Use the last direction that was successful (conflict resolution)
		if existingObj.DirectionLast == "target_to_source" {
			direction = "source_to_target"
		} else {
			direction = "target_to_source"
		}
	}

	// Apply the sync
	if direction == "source_to_target" {
		err = withRetry(config.RetryPolicy{
			MaxRetries:     3,
			InitialBackoff: 1000,
			MaxBackoff:     60000,
			Multiplier:     2.0,
		}, func() error {
			return targetBackend.SetSecret(secretName, sourceSecret.Value)
		})
		if err != nil {
			obj := &config.SyncObject{
				SyncID:        syncID,
				SourceVaultID: sourceID,
				TargetVaultID: targetID,
				SecretName:    secretName,
				SourceChecksum: sourceChecksum,
				TargetChecksum: targetChecksum,
				LastSyncTime:  time.Now().Unix(),
				LastSyncStatus: "failed",
				LastSyncError: err.Error(),
				DirectionLast: "source_to_target",
			}
			e.store.UpsertSyncObject(obj)
			return err
		}
		targetChecksum = sourceChecksum
	} else {
		err = withRetry(config.RetryPolicy{
			MaxRetries:     3,
			InitialBackoff: 1000,
			MaxBackoff:     60000,
			Multiplier:     2.0,
		}, func() error {
			return sourceBackend.SetSecret(secretName, targetSecret.Value)
		})
		if err != nil {
			obj := &config.SyncObject{
				SyncID:        syncID,
				SourceVaultID: sourceID,
				TargetVaultID: targetID,
				SecretName:    secretName,
				SourceChecksum: sourceChecksum,
				TargetChecksum: targetChecksum,
				LastSyncTime:  time.Now().Unix(),
				LastSyncStatus: "failed",
				LastSyncError: err.Error(),
				DirectionLast: "target_to_source",
			}
			e.store.UpsertSyncObject(obj)
			return err
		}
		sourceChecksum = targetChecksum
	}

	// Record success
	obj := &config.SyncObject{
		SyncID:        syncID,
		SourceVaultID: sourceID,
		TargetVaultID: targetID,
		SecretName:    secretName,
		SourceChecksum: sourceChecksum,
		TargetChecksum: targetChecksum,
		LastSyncTime:  time.Now().Unix(),
		LastSyncStatus: "success",
		DirectionLast: direction,
	}
	e.store.UpsertSyncObject(obj)

	return nil
}

// filterSecrets filters secrets based on patterns
func filterSecrets(secrets []string, filter config.FilterConfig) []string {
	if len(filter.Patterns) == 0 && len(filter.Exclude) == 0 {
		return secrets
	}

	var filtered []string
	for _, secret := range secrets {
		excluded := false

		// Check exclude patterns
		for _, pattern := range filter.Exclude {
			if matchPattern(pattern, secret) {
				excluded = true
				break
			}
		}

		if excluded {
			continue
		}

		// If patterns are defined, secret must match at least one
		if len(filter.Patterns) > 0 {
			matched := false
			for _, pattern := range filter.Patterns {
				if matchPattern(pattern, secret) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		filtered = append(filtered, secret)
	}

	return filtered
}

// matchPattern matches a glob pattern
func matchPattern(pattern, name string) bool {
	// Simple glob matching: * matches anything
	// For more complex patterns, use github.com/gobwas/glob
	if pattern == "*" {
		return true
	}
	if pattern == name {
		return true
	}
	// Simple wildcard matching
	if len(pattern) > 0 && pattern[0] == '*' && len(name) >= len(pattern)-1 {
		return name[len(name)-(len(pattern)-1):] == pattern[1:]
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		return name[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	return false
}

// hashString creates an MD5 hash of a string
func hashString(s string) string {
	hash := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", hash)
}

// withRetry executes a function with exponential backoff retry
func withRetry(policy config.RetryPolicy, fn func() error) error {
	var lastErr error
	backoff := policy.InitialBackoff

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < policy.MaxRetries {
			time.Sleep(time.Duration(backoff) * time.Millisecond)
			backoff = int(float64(backoff) * policy.Multiplier)
			if backoff > policy.MaxBackoff {
				backoff = policy.MaxBackoff
			}
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SyncResult holds the results of a sync operation
type SyncResult struct {
	Success int
	Failure int
}

// executeSyncUnidirectionalConcurrent syncs secrets concurrently in one direction
func (e *Engine) executeSyncUnidirectionalConcurrent(syncCfg *config.SyncConfig, sourceBackend vault.Backend, workerCount int) SyncResult {
	secrets, err := sourceBackend.ListSecrets()
	if err != nil {
		e.logger.Error("failed to list secrets", slog.String("error", err.Error()))
		return SyncResult{Success: 0, Failure: 0}
	}

	filteredSecrets := filterSecrets(secrets, syncCfg.Filter)
	
	// Create a semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, workerCount)
	defer close(semaphore)

	var wg sync.WaitGroup
	results := make(chan bool, len(filteredSecrets)*len(syncCfg.Targets))
	
	// Process each secret for each target
	for _, secretName := range filteredSecrets {
		for _, targetID := range syncCfg.Targets {
			wg.Add(1)
			go func(secret, target string) {
				defer wg.Done()
				semaphore <- struct{}{}        // Acquire
				defer func() { <-semaphore }() // Release

				err := e.syncSecretUnidirectional(syncCfg.ID, syncCfg.Source, target, secret, sourceBackend)
				if err != nil {
					e.logger.Error("failed to sync secret",
						slog.String("sync_id", syncCfg.ID),
						slog.String("secret", secret),
						slog.String("target", target),
						slog.String("error", err.Error()),
					)
					results <- false
				} else {
					results <- true
				}
			}(secretName, targetID)
		}
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	success := 0
	failure := 0
	for result := range results {
		if result {
			success++
		} else {
			failure++
		}
	}

	return SyncResult{Success: success, Failure: failure}
}

// executeSyncBidirectionalConcurrent syncs secrets bidirectionally with concurrency
func (e *Engine) executeSyncBidirectionalConcurrent(syncCfg *config.SyncConfig, sourceBackend, targetBackend vault.Backend, workerCount int) SyncResult {
	secrets, err := sourceBackend.ListSecrets()
	if err != nil {
		e.logger.Error("failed to list secrets", slog.String("error", err.Error()))
		return SyncResult{Success: 0, Failure: 0}
	}

	filteredSecrets := filterSecrets(secrets, syncCfg.Filter)
	
	// Create a semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, workerCount)
	defer close(semaphore)

	var wg sync.WaitGroup
	results := make(chan bool, len(filteredSecrets))
	
	// Process each secret concurrently
	for _, secretName := range filteredSecrets {
		wg.Add(1)
		go func(secret string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			err := e.syncSecretBidirectional(syncCfg.ID, syncCfg.Source, syncCfg.Targets[0], secret, sourceBackend, targetBackend)
			if err != nil {
				e.logger.Error("failed to sync secret",
					slog.String("sync_id", syncCfg.ID),
					slog.String("secret", secret),
					slog.String("error", err.Error()),
				)
				results <- false
			} else {
				results <- true
			}
		}(secretName)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	success := 0
	failure := 0
	for result := range results {
		if result {
			success++
		} else {
			failure++
		}
	}

	return SyncResult{Success: success, Failure: failure}
}
