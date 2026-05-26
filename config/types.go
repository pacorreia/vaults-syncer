package config

// Config represents the top-level configuration.
type Config struct {
	Vaults  []VaultConfig `yaml:"vaults"  json:"vaults"`
	Syncs   []SyncConfig  `yaml:"syncs"   json:"syncs"`
	Server  ServerConfig  `yaml:"server"  json:"server"`
	Logging LoggingConfig `yaml:"logging" json:"logging"`
}

// OAuthConfig defines OAuth 2.0 authentication.
type OAuthConfig struct {
	TokenEndpoint string            `yaml:"token_endpoint" json:"token_endpoint"` // Token exchange endpoint
	ClientID      string            `yaml:"client_id"      json:"client_id"`
	ClientSecret  string            `yaml:"client_secret"  json:"client_secret"`
	Scope         string            `yaml:"scope"          json:"scope"`
	ExtraParams   map[string]string `yaml:"extra_params"   json:"extra_params"` // Device ID, custom params, etc.
}

// AuthConfig defines authentication method and credentials.
type AuthConfig struct {
	Method  string            `yaml:"method"  json:"method"`  // bearer, basic, oauth2, api_key, custom
	Headers map[string]string `yaml:"headers" json:"headers"` // Auth-specific headers
	OAuth   *OAuthConfig      `yaml:"oauth"   json:"oauth"`   // OAuth 2.0 config
}

// ResponseParserConfig defines how to extract data from responses.
type ResponseParserConfig struct {
	ListPath  string `yaml:"path"       json:"path"`       // JSONPath to array of items (e.g., "data" or "value")
	NameField string `yaml:"name_field" json:"name_field"` // Field containing secret name
	ValuePath string `yaml:"value_path" json:"value_path"` // JSONPath to extract value
}

// OperationConfig defines vault-specific behavior for an operation.
type OperationConfig struct {
	Method         string                `yaml:"method"       json:"method"`       // GET, POST, PUT, DELETE
	Endpoint       string                `yaml:"endpoint"     json:"endpoint"`     // Optional: override endpoint
	StatusCodes    []int                 `yaml:"status_codes" json:"status_codes"` // Success status codes
	ResponseParser *ResponseParserConfig `yaml:"response"     json:"response"`     // How to parse response
}

// ExternalToolConfig represents the contents of a per-tool YAML configuration file.
// It defines CLI commands that are executed to perform vault operations.
type ExternalToolConfig struct {
	// Env holds environment variables injected into every command execution.
	// Values support ${VAR} substitution, which is expanded once at config load time.
	Env map[string]string `yaml:"env" json:"env"`
	// EnvPassthrough is a list of environment variable names whose current runtime
	// values are forwarded to every command execution. Unlike Env, values are read
	// from the process environment at the moment each command runs, so they pick up
	// rotated credentials or tokens without restarting the daemon.
	EnvPassthrough []string `yaml:"env_passthrough" json:"env_passthrough"`
	// Operations maps operation names (list, get, set, delete, test) to their command definitions.
	Operations map[string]*ToolOperationConfig `yaml:"operations" json:"operations"`
}

// ToolOperationConfig defines the CLI command used for a single vault operation.
type ToolOperationConfig struct {
	// Command is the executable to run (e.g. "aws", "vault").
	Command string `yaml:"command" json:"command"`
	// Args are the arguments passed to the command. Supports Go template variables:
	// {{.Name}} for the secret name and {{.Value}} for the secret value.
	Args []string `yaml:"args" json:"args"`
	// Output describes how to interpret the command's stdout.
	Output ToolOutputConfig `yaml:"output" json:"output"`
	// SuccessExitCodes lists exit codes that indicate success. Defaults to [0].
	SuccessExitCodes []int `yaml:"success_exit_codes" json:"success_exit_codes"`
}

// ToolOutputConfig describes how to parse the stdout of a CLI command.
type ToolOutputConfig struct {
	// Format is the output format: "json", "text", or "lines". Defaults to "json".
	Format string `yaml:"format" json:"format"`
	// Path is a dot-notation path into the JSON output (e.g. "SecretList" or "data.keys").
	Path string `yaml:"path" json:"path"`
	// NameField is the JSON field within each list item that holds the secret name.
	NameField string `yaml:"name_field" json:"name_field"`
}

// VaultConfig represents a vault definition.
type VaultConfig struct {
	ID   string `yaml:"id"   json:"id"`
	Name string `yaml:"name" json:"name"`
	// Type is the vault type: vaultwarden, vault, azure, aws, generic, or tool.
	Type               string                      `yaml:"type"               json:"type"`
	Endpoint           string                      `yaml:"endpoint"           json:"endpoint"`
	Method             string                      `yaml:"method"             json:"method"` // PUT or POST (legacy, use operations_override)
	Auth               *AuthConfig                 `yaml:"auth"               json:"auth"`   // Structured authentication
	FieldNames         FieldNamesConfig            `yaml:"field_names"        json:"field_names"`
	Headers            map[string]string           `yaml:"headers"            json:"headers"`
	OperationsOverride map[string]*OperationConfig `yaml:"operations_override" json:"operations_override"` // list, get, set, delete
	Timeout            int                         `yaml:"timeout"            json:"timeout"`              // seconds
	SkipSSLVerify      bool                        `yaml:"skip_ssl_verify"    json:"skip_ssl_verify"`
	// ToolConfig is the path to an ExternalToolConfig YAML file (required when Type is "tool").
	ToolConfig string `yaml:"tool_config,omitempty" json:"tool_config,omitempty"`

	// ResolvedTool is populated by LoadConfig from the file referenced by ToolConfig.
	// It is not read from YAML directly.
	ResolvedTool *ExternalToolConfig `yaml:"-" json:"-"`

	// Legacy fields - NO LONGER SUPPORTED, will cause validation error
	LegacyAuthMethod  string            `yaml:"auth_method,omitempty"  json:"-"`
	LegacyAuthHeaders map[string]string `yaml:"auth_headers,omitempty" json:"-"`
}

// FieldNamesConfig defines how secrets are structured in requests.
type FieldNamesConfig struct {
	NameField  string `yaml:"name_field"  json:"name_field"`
	ValueField string `yaml:"value_field" json:"value_field"`
}

// SyncConfig defines a sync relationship between vaults.
type SyncConfig struct {
	ID                string       `yaml:"id"                 json:"id"`
	Source            string       `yaml:"source"             json:"source"`    // vault ID
	Targets           []string     `yaml:"targets"            json:"targets"`   // list of vault IDs
	SyncType          string       `yaml:"sync_type"          json:"sync_type"` // unidirectional or bidirectional
	Schedule          string       `yaml:"schedule"           json:"schedule"`  // cron expression
	Filter            FilterConfig `yaml:"filter"             json:"filter"`
	Transforms        []Transform  `yaml:"transforms"         json:"transforms"`
	RetryPolicy       RetryPolicy  `yaml:"retry_policy"       json:"retry_policy"`
	ConcurrentWorkers int          `yaml:"concurrent_workers" json:"concurrent_workers"` // number of parallel workers (0 = sequential)
	// Enabled controls whether this sync is active. Defaults to true when omitted.
	// Use a pointer so that an explicit `enabled: false` in YAML is preserved and
	// not overwritten by the defaulting logic.
	Enabled *bool `yaml:"enabled" json:"enabled"`
}

// IsEnabled reports whether the sync is enabled. Returns true when the field
// is omitted from YAML (nil pointer) so that syncs are active by default.
func (s *SyncConfig) IsEnabled() bool {
	return s.Enabled == nil || *s.Enabled
}

// FilterConfig allows filtering which secrets to sync.
type FilterConfig struct {
	Patterns []string `yaml:"patterns" json:"patterns"` // glob patterns for secret names
	Exclude  []string `yaml:"exclude"  json:"exclude"`
}

// Transform applies transformations to secret values during sync.
type Transform struct {
	Field string `yaml:"field" json:"field"`
	Type  string `yaml:"type"  json:"type"` // base64_encode, base64_decode, etc.
	Value string `yaml:"value" json:"value"`
}

// RetryPolicy defines retry behavior for failed syncs.
type RetryPolicy struct {
	MaxRetries     int     `yaml:"max_retries"     json:"max_retries"`
	InitialBackoff int     `yaml:"initial_backoff" json:"initial_backoff"` // milliseconds
	MaxBackoff     int     `yaml:"max_backoff"     json:"max_backoff"`
	Multiplier     float64 `yaml:"multiplier"      json:"multiplier"`
}

// ServerConfig defines HTTP server settings.
type ServerConfig struct {
	Port           int    `yaml:"port"            json:"port"`
	Address        string `yaml:"address"         json:"address"`
	MetricsPort    int    `yaml:"metrics_port"    json:"metrics_port"`
	MetricsAddress string `yaml:"metrics_address" json:"metrics_address"`
}

// LoggingConfig defines logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level"  json:"level"`  // debug, info, warn, error
	Format string `yaml:"format" json:"format"` // json or text
}

// SyncObject represents a secret object in the sync database.
type SyncObject struct {
	ID             int64
	SyncID         string
	SourceVaultID  string
	TargetVaultID  string
	SecretName     string
	ExternalID     string
	SourceChecksum string
	TargetChecksum string
	LastSyncTime   int64
	LastSyncStatus string
	LastSyncError  string
	SyncCount      int
	FailureCount   int
	DirectionLast  string // source_to_target or target_to_source
}

// Helper methods for VaultConfig.

// GetVaultType returns the vault type, defaulting to "generic".
func (vc *VaultConfig) GetVaultType() string {
	if vc.Type != "" {
		return vc.Type
	}
	return "generic"
}

// PopulateDefaults initializes optional fields with sensible defaults.
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
