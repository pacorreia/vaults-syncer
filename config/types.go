package config

// Config represents the top-level configuration
type Config struct {
	Vaults []VaultConfig `yaml:"vaults"`
	Syncs  []SyncConfig  `yaml:"syncs"`
	Server ServerConfig  `yaml:"server"`
	Logging LoggingConfig `yaml:"logging"`
}

// OAuthConfig defines OAuth 2.0 authentication
type OAuthConfig struct {
	TokenEndpoint string            `yaml:"token_endpoint"` // Token exchange endpoint
	ClientID      string            `yaml:"client_id"`
	ClientSecret  string            `yaml:"client_secret"`
	Scope         string            `yaml:"scope"`
	ExtraParams   map[string]string `yaml:"extra_params"` // Device ID, custom params, etc.
}

// AuthConfig defines authentication method and credentials
type AuthConfig struct {
	Method       string            `yaml:"method"` // bearer, basic, oauth2, api_key, custom
	Headers      map[string]string `yaml:"headers"` // Auth-specific headers
	OAuth        *OAuthConfig      `yaml:"oauth"`   // OAuth 2.0 config
}

// ResponseParserConfig defines how to extract data from responses
type ResponseParserConfig struct {
	ListPath  string `yaml:"path"`        // JSONPath to array of items (e.g., "data" or "value")
	NameField string `yaml:"name_field"`  // Field containing secret name
	ValuePath string `yaml:"value_path"`  // JSONPath to extract value
}

// OperationConfig defines vault-specific behavior for an operation
type OperationConfig struct {
	Method         string                 `yaml:"method"`          // GET, POST, PUT, DELETE
	Endpoint       string                 `yaml:"endpoint"`        // Optional: override endpoint
	StatusCodes    []int                  `yaml:"status_codes"`    // Success status codes
	ResponseParser *ResponseParserConfig  `yaml:"response"`        // How to parse response
}

// VaultConfig represents a vault definition
type VaultConfig struct {
	ID                  string              `yaml:"id"`
	Name                string              `yaml:"name"`
	Type                string              `yaml:"type"` // vaultwarden, vault, azure, aws, generic
	Endpoint            string              `yaml:"endpoint"`
	Method              string              `yaml:"method"` // PUT or POST (legacy, use operations_override)
	Auth                *AuthConfig         `yaml:"auth"`   // Structured authentication
	FieldNames          FieldNamesConfig    `yaml:"field_names"`
	Headers             map[string]string   `yaml:"headers"`
	OperationsOverride  map[string]*OperationConfig `yaml:"operations_override"` // list, get, set, delete
	Timeout             int                 `yaml:"timeout"` // seconds
	SkipSSLVerify       bool                `yaml:"skip_ssl_verify"`
	
	// Legacy fields - NO LONGER SUPPORTED, will cause validation error
	LegacyAuthMethod    string              `yaml:"auth_method,omitempty" json:"-"`
	LegacyAuthHeaders   map[string]string   `yaml:"auth_headers,omitempty" json:"-"`
}

// FieldNamesConfig defines how secrets are structured in requests
type FieldNamesConfig struct {
	NameField  string `yaml:"name_field"`
	ValueField string `yaml:"value_field"`
}

// SyncConfig defines a sync relationship between vaults
type SyncConfig struct {
	ID                string        `yaml:"id"`
	Source            string        `yaml:"source"` // vault ID
	Targets           []string      `yaml:"targets"` // list of vault IDs
	SyncType          string        `yaml:"sync_type"` // unidirectional or bidirectional
	Schedule          string        `yaml:"schedule"` // cron expression
	Filter            FilterConfig  `yaml:"filter"`
	Transforms        []Transform   `yaml:"transforms"`
	RetryPolicy       RetryPolicy   `yaml:"retry_policy"`
	ConcurrentWorkers int           `yaml:"concurrent_workers"` // number of parallel workers (0 = sequential)
	Enabled           bool          `yaml:"enabled"`
}

// FilterConfig allows filtering which secrets to sync
type FilterConfig struct {
	Patterns []string `yaml:"patterns"` // glob patterns for secret names
	Exclude  []string `yaml:"exclude"`
}

// Transform applies transformations to secret values during sync
type Transform struct {
	Field string `yaml:"field"`
	Type  string `yaml:"type"` // base64_encode, base64_decode, etc.
	Value string `yaml:"value"`
}

// RetryPolicy defines retry behavior for failed syncs
type RetryPolicy struct {
	MaxRetries     int `yaml:"max_retries"`
	InitialBackoff int `yaml:"initial_backoff"` // milliseconds
	MaxBackoff     int `yaml:"max_backoff"`
	Multiplier     float64 `yaml:"multiplier"`
}

// ServerConfig defines HTTP server settings
type ServerConfig struct {
	Port            int    `yaml:"port"`
	Address         string `yaml:"address"`
	MetricsPort     int    `yaml:"metrics_port"`
	MetricsAddress  string `yaml:"metrics_address"`
}

// LoggingConfig defines logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"` // debug, info, warn, error
	Format string `yaml:"format"` // json or text
}

// SyncObject represents a secret object in the sync database
type SyncObject struct {
	ID               int64
	SyncID           string
	SourceVaultID    string
	TargetVaultID    string
	SecretName       string
	ExternalID       string
	SourceChecksum   string
	TargetChecksum   string
	LastSyncTime     int64
	LastSyncStatus   string
	LastSyncError    string
	SyncCount        int
	FailureCount     int
	DirectionLast    string // source_to_target or target_to_source
}

// Helper methods for VaultConfig

// GetVaultType returns the vault type, defaulting to "generic"
func (vc *VaultConfig) GetVaultType() string {
	if vc.Type != "" {
		return vc.Type
	}
	return "generic"
}

// PopulateDefaults initializes optional fields with sensible defaults
func (vc *VaultConfig) PopulateDefaults() {
	// Initialize optional maps if nil
	if vc.Auth != nil && vc.Auth.Headers == nil {
		vc.Auth.Headers = make(map[string]string)
	}
	if vc.Headers == nil {
		vc.Headers = make(map[string]string)
	}
	
	// Default Type to generic if not specified
	if vc.Type == "" {
		vc.Type = "generic"
	}
}
