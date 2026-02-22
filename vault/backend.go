package vault

import (
	"github.com/pacorreia/vaults-syncer/config"
)

// BackendCapabilities describes what operations a backend supports
type BackendCapabilities struct {
	CanList   bool // Can list all secret names
	CanGet    bool // Can retrieve secret values
	CanSet    bool // Can create/update secrets
	CanDelete bool // Can delete secrets
	CanSync   bool // Supports bidirectional sync
}

// Backend is the interface that all vault implementations must satisfy.
// This abstraction allows supporting multiple vault types (Vaultwarden, HashiCorp Vault, Azure, AWS, etc.)
// with a consistent API for the sync engine.
type Backend interface {
	// ListSecrets returns a list of all secret names in the vault
	ListSecrets() ([]string, error)

	// GetSecret retrieves a secret by name
	GetSecret(name string) (*Secret, error)

	// SetSecret creates or updates a secret
	SetSecret(name, value string) error

	// DeleteSecret removes a secret from the vault
	DeleteSecret(name string) error

	// TestConnection verifies that the backend is accessible and credentials are valid
	TestConnection() error

	// Type returns the vault type (e.g., "vaultwarden", "vault", "azure", "aws")
	Type() string

	// Capabilities returns what operations this backend supports
	Capabilities() BackendCapabilities
}

// GenericBackend wraps the generic HTTP-based Client to implement the Backend interface.
// This is the default implementation used for most vault types.
type GenericBackend struct {
	client *Client
}

// NewGenericBackend creates a new GenericBackend wrapping the given client
func NewGenericBackend(client *Client) *GenericBackend {
	return &GenericBackend{
		client: client,
	}
}

// ListSecrets implements Backend.ListSecrets
func (b *GenericBackend) ListSecrets() ([]string, error) {
	return b.client.ListSecrets()
}

// GetSecret implements Backend.GetSecret
func (b *GenericBackend) GetSecret(name string) (*Secret, error) {
	return b.client.GetSecret(name)
}

// SetSecret implements Backend.SetSecret
func (b *GenericBackend) SetSecret(name, value string) error {
	return b.client.SetSecret(name, value)
}

// DeleteSecret implements Backend.DeleteSecret
func (b *GenericBackend) DeleteSecret(name string) error {
	return b.client.DeleteSecret(name)
}

// TestConnection implements Backend.TestConnection
func (b *GenericBackend) TestConnection() error {
	return b.client.TestConnection()
}

// Type implements Backend.Type
func (b *GenericBackend) Type() string {
	return b.client.cfg.GetVaultType()
}

// Capabilities implements Backend.Capabilities
func (b *GenericBackend) Capabilities() BackendCapabilities {
	// Generic HTTP-based backends support all operations
	return BackendCapabilities{
		CanList:   true,
		CanGet:    true,
		CanSet:    true,
		CanDelete: true,
		CanSync:   true,
	}
}

// NewBackend creates a Backend for the given vault configuration.
// This is the main factory function that determines which backend implementation to use.
func NewBackend(cfg *config.VaultConfig) (Backend, error) {
	// Initialize optional fields with defaults
	cfg.PopulateDefaults()

	// Create the underlying HTTP client
	client := NewClient(cfg)

	// For now, all vault types use the GenericBackend.
	// In the future, we can add specialized implementations:
	// - HashiCorpVaultBackend for native Vault SDK
	// - AzureKeyVaultBackend for Azure SDK
	// - AWSSecretsManagerBackend for AWS SDK
	vaultType := cfg.GetVaultType()
	switch vaultType {
	case "vaultwarden", "bitwarden", "keeper", "vault", "azure", "aws", "generic":
		return NewGenericBackend(client), nil
	default:
		// Unknown types still get GenericBackend (best effort)
		return NewGenericBackend(client), nil
	}
}
