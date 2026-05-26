package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/pacorreia/vaults-syncer/config"
	"github.com/pacorreia/vaults-syncer/storage"
)

// Handler manages HTTP handlers
type Handler struct {
	runner  Runner
	store   *storage.Store
	cfg     *config.Config
	cfgMu   sync.RWMutex
	logger  *slog.Logger
}

// Runner defines the runner behaviors required by the API handler.
type Runner interface {
	IsRunning() bool
	GetSyncStatus(syncID string, store *storage.Store) (map[string]interface{}, error)
	ExecuteSyncNow(syncID string, cfg *config.Config) error
}

// NewHandler creates a new handler
func NewHandler(runner Runner, store *storage.Store, cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		runner: runner,
		store:  store,
		cfg:    cfg,
		logger: logger,
	}
}

// SetConfig atomically updates the handler's active configuration. It is safe
// to call concurrently with in-flight requests.
func (h *Handler) SetConfig(cfg *config.Config) {
	h.cfgMu.Lock()
	defer h.cfgMu.Unlock()
	h.cfg = cfg
}

// SetRunner atomically replaces the active runner. It must be called after
// reloadConfig so that ExecuteSync uses the engine that knows about the latest
// vault backends.
func (h *Handler) SetRunner(r Runner) {
	h.cfgMu.Lock()
	defer h.cfgMu.Unlock()
	h.runner = r
}

// getConfig returns the current configuration (thread-safe).
func (h *Handler) getConfig() *config.Config {
	h.cfgMu.RLock()
	defer h.cfgMu.RUnlock()
	return h.cfg
}

// getRunner returns the current runner (thread-safe).
func (h *Handler) getRunner() Runner {
	h.cfgMu.RLock()
	defer h.cfgMu.RUnlock()
	return h.runner
}

// Health handles health check requests
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	cfg := h.getConfig()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"status":  "healthy",
		"running": h.getRunner().IsRunning(),
		"syncs":   len(cfg.Syncs),
		"vaults":  len(cfg.Vaults),
	}

	json.NewEncoder(w).Encode(response)
}

// GetSyncStatus handles status requests for a specific sync
func (h *Handler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	syncID := r.PathValue("sync_id")
	if syncID == "" {
		http.Error(w, "sync_id is required", http.StatusBadRequest)
		return
	}

	status, err := h.getRunner().GetSyncStatus(syncID, h.store)
	if err != nil {
		h.logger.Error("failed to get sync status",
			slog.String("sync_id", syncID),
			slog.String("error", err.Error()),
		)
		http.Error(w, fmt.Sprintf("failed to get status: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// ListSyncs handles listing all syncs
func (h *Handler) ListSyncs(w http.ResponseWriter, r *http.Request) {
	cfg := h.getConfig()
	syncs := make([]map[string]interface{}, 0)

	for _, syncCfg := range cfg.Syncs {
		syncMap := map[string]interface{}{
			"id":       syncCfg.ID,
			"source":   syncCfg.Source,
			"targets":  syncCfg.Targets,
			"type":     syncCfg.SyncType,
			"schedule": syncCfg.Schedule,
			"enabled":  syncCfg.IsEnabled(),
		}
		syncs = append(syncs, syncMap)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"syncs": syncs})
}

// ExecuteSync handles manual sync execution
func (h *Handler) ExecuteSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	syncID := r.PathValue("sync_id")
	if syncID == "" {
		http.Error(w, "sync_id is required", http.StatusBadRequest)
		return
	}

	cfg := h.getConfig()
	runner := h.getRunner()
	// Execute sync asynchronously
	go func() {
		if err := runner.ExecuteSyncNow(syncID, cfg); err != nil {
			h.logger.Error("manual sync execution failed",
				slog.String("sync_id", syncID),
				slog.String("error", err.Error()),
			)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sync_id": syncID,
		"status":  "executing",
	})
}

// GetMetrics handles Prometheus-style metrics requests
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	cfg := h.getConfig()
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	// Simple metrics output
	metrics := "# HELP syncs_configured Total number of syncs configured\n"
	metrics += fmt.Sprintf("# TYPE syncs_configured gauge\nsyncs_configured %d\n", len(cfg.Syncs))

	metrics += "# HELP syncs_enabled Total number of enabled syncs\n"
	metrics += "# TYPE syncs_enabled gauge\n"
	enabledCount := 0
	for _, s := range cfg.Syncs {
		if s.IsEnabled() {
			enabledCount++
		}
	}
	metrics += fmt.Sprintf("syncs_enabled %d\n", enabledCount)

	metrics += "# HELP runner_running Whether the sync runner is running\n"
	metrics += "# TYPE runner_running gauge\n"
	runningVal := 0
	if h.runner.IsRunning() {
		runningVal = 1
	}
	metrics += fmt.Sprintf("runner_running %d\n", runningVal)

	w.Write([]byte(metrics))
}

// ListVaults handles listing all configured vaults (without sensitive auth data)
func (h *Handler) ListVaults(w http.ResponseWriter, r *http.Request) {
	cfg := h.getConfig()
	vaults := make([]map[string]interface{}, 0, len(cfg.Vaults))
	for _, v := range cfg.Vaults {
		vaults = append(vaults, map[string]interface{}{
			"id":       v.ID,
			"name":     v.Name,
			"type":     v.Type,
			"endpoint": v.Endpoint,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"vaults": vaults})
}

// GetSyncRuns handles retrieving the run history for a specific sync
func (h *Handler) GetSyncRuns(w http.ResponseWriter, r *http.Request) {
	syncID := r.PathValue("sync_id")
	if syncID == "" {
		http.Error(w, "sync_id is required", http.StatusBadRequest)
		return
	}

	limit := 20
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	runs, err := h.store.GetSyncRuns(syncID, limit)
	if err != nil {
		h.logger.Error("failed to get sync runs",
			slog.String("sync_id", syncID),
			slog.String("error", err.Error()),
		)
		http.Error(w, fmt.Sprintf("failed to get runs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"sync_id": syncID, "runs": runs})
}
