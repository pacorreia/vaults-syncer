package vault

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
)

// Client handles communication with a vault
type Client struct {
	cfg          *config.VaultConfig
	client       *http.Client
	parser       ResponseParser // NEW: Configurable response parser
	oauthToken   string
	oauthExpires time.Time
}

// Secret represents a secret with name and value
type Secret struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NewClient creates a new vault client
func NewClient(cfg *config.VaultConfig) *Client {
	// Initialize optional fields with defaults
	cfg.PopulateDefaults()
	applyDefaultOperations(cfg)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.SkipSSLVerify,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
	}

	// Initialize parser based on vault type or config override
	var parser ResponseParser
	if cfg.OperationsOverride != nil && cfg.OperationsOverride["list"] != nil && cfg.OperationsOverride["list"].ResponseParser != nil {
		parser = NewParserFromConfig(cfg.OperationsOverride["list"].ResponseParser)
	} else {
		parser = GetParserForVaultType(cfg.GetVaultType())
	}

	return &Client{
		cfg:    cfg,
		client: client,
		parser: parser,
	}
}

func applyDefaultOperations(cfg *config.VaultConfig) {
	if cfg.OperationsOverride == nil {
		cfg.OperationsOverride = make(map[string]*config.OperationConfig)
	}

	switch strings.ToLower(cfg.GetVaultType()) {
	case "azure":
		base := strings.TrimSuffix(cfg.Endpoint, "/")
		listEndpoint := fmt.Sprintf("%s?api-version=7.4", base)
		secretEndpoint := fmt.Sprintf("%s/{name}?api-version=7.4", base)
		mergeOperationDefaults(cfg, "list", &config.OperationConfig{
			Method:   http.MethodGet,
			Endpoint: listEndpoint,
			ResponseParser: &config.ResponseParserConfig{
				ListPath:  "value",
				NameField: "name",
			},
		})
		mergeOperationDefaults(cfg, "get", &config.OperationConfig{
			Method:   http.MethodGet,
			Endpoint: secretEndpoint,
			ResponseParser: &config.ResponseParserConfig{
				ValuePath: "value",
			},
		})
		mergeOperationDefaults(cfg, "set", &config.OperationConfig{
			Method:      http.MethodPut,
			Endpoint:    secretEndpoint,
			StatusCodes: []int{http.StatusOK, http.StatusCreated},
		})
		mergeOperationDefaults(cfg, "delete", &config.OperationConfig{
			Method:      http.MethodDelete,
			Endpoint:    secretEndpoint,
			StatusCodes: []int{http.StatusOK, http.StatusNoContent},
		})

	case "vault":
		base := strings.TrimSuffix(cfg.Endpoint, "/")
		listEndpoint := hashiCorpListEndpoint(base)
		dataEndpoint := fmt.Sprintf("%s/{name}", base)
		deleteEndpoint := hashiCorpDeleteEndpoint(base)
		mergeOperationDefaults(cfg, "list", &config.OperationConfig{
			Method:   http.MethodGet,
			Endpoint: listEndpoint,
			ResponseParser: &config.ResponseParserConfig{
				ListPath:  "data.keys",
				NameField: "key",
			},
		})
		mergeOperationDefaults(cfg, "get", &config.OperationConfig{
			Method:   http.MethodGet,
			Endpoint: dataEndpoint,
			ResponseParser: &config.ResponseParserConfig{
				ValuePath: "data.data",
			},
		})
		mergeOperationDefaults(cfg, "set", &config.OperationConfig{
			Method:      http.MethodPost,
			Endpoint:    dataEndpoint,
			StatusCodes: []int{http.StatusOK, http.StatusCreated, http.StatusNoContent},
		})
		mergeOperationDefaults(cfg, "delete", &config.OperationConfig{
			Method:      http.MethodDelete,
			Endpoint:    deleteEndpoint,
			StatusCodes: []int{http.StatusOK, http.StatusNoContent},
		})

	case "bitwarden", "vaultwarden":
		// Vaultwarden/Bitwarden list endpoint (GET /api/ciphers)
		mergeOperationDefaults(cfg, "list", &config.OperationConfig{
			Method: http.MethodGet,
			ResponseParser: &config.ResponseParserConfig{
				ListPath:  "data",
				NameField: "name",
			},
		})
		// Vaultwarden/Bitwarden set endpoint (POST /api/ciphers - no {name} in path)
		// The cipher object is created with the name field, not in the URL path
		mergeOperationDefaults(cfg, "set", &config.OperationConfig{
			Method:      http.MethodPost,
			Endpoint:    strings.TrimSuffix(cfg.Endpoint, "/"),
			StatusCodes: []int{http.StatusOK, http.StatusCreated},
		})
		// Vaultwarden/Bitwarden delete endpoint (DELETE /api/ciphers/{id})
		// Note: Assumes secret name matches cipher ID for simplicity
		mergeOperationDefaults(cfg, "delete", &config.OperationConfig{
			Method:      http.MethodDelete,
			Endpoint:    fmt.Sprintf("%s/{name}", strings.TrimSuffix(cfg.Endpoint, "/")),
			StatusCodes: []int{http.StatusOK, http.StatusNoContent},
		})

	case "keeper":
		mergeOperationDefaults(cfg, "list", &config.OperationConfig{
			Method: http.MethodGet,
			ResponseParser: &config.ResponseParserConfig{
				ListPath:  "records",
				NameField: "title",
			},
		})
		mergeOperationDefaults(cfg, "get", &config.OperationConfig{
			Method: http.MethodGet,
			ResponseParser: &config.ResponseParserConfig{
				ValuePath: "data",
			},
		})
	}
}

func mergeOperationDefaults(cfg *config.VaultConfig, name string, defaults *config.OperationConfig) {
	if defaults == nil {
		return
	}
	current := cfg.OperationsOverride[name]
	if current == nil {
		cfg.OperationsOverride[name] = defaults
		return
	}
	if current.Method == "" {
		current.Method = defaults.Method
	}
	if current.Endpoint == "" {
		current.Endpoint = defaults.Endpoint
	}
	if len(current.StatusCodes) == 0 && len(defaults.StatusCodes) > 0 {
		current.StatusCodes = defaults.StatusCodes
	}
	if current.ResponseParser == nil && defaults.ResponseParser != nil {
		current.ResponseParser = defaults.ResponseParser
	}
}

func hashiCorpListEndpoint(base string) string {
	listBase := base
	if strings.Contains(listBase, "/data/") {
		listBase = strings.Replace(listBase, "/data/", "/metadata/", 1)
	} else if strings.HasSuffix(listBase, "/data") {
		listBase = strings.TrimSuffix(listBase, "/data") + "/metadata"
	}
	listBase = strings.TrimSuffix(listBase, "/")
	return fmt.Sprintf("%s?list=true", listBase)
}

func hashiCorpDeleteEndpoint(base string) string {
	deleteBase := base
	if strings.Contains(deleteBase, "/data/") {
		deleteBase = strings.Replace(deleteBase, "/data/", "/metadata/", 1)
	} else if strings.HasSuffix(deleteBase, "/data") {
		deleteBase = strings.TrimSuffix(deleteBase, "/data") + "/metadata"
	}
	return fmt.Sprintf("%s/{name}", strings.TrimSuffix(deleteBase, "/"))
}

func (c *Client) operationConfig(op string) *config.OperationConfig {
	if c.cfg.OperationsOverride == nil {
		return nil
	}
	return c.cfg.OperationsOverride[op]
}

func (c *Client) operationEndpoint(op string, name string) string {
	defaultEndpoint := strings.TrimSuffix(c.cfg.Endpoint, "/")
	if op == "delete" || op == "get" || op == "set" {
		defaultEndpoint = fmt.Sprintf("%s/%s", defaultEndpoint, url.PathEscape(name))
	}

	if opCfg := c.operationConfig(op); opCfg != nil && opCfg.Endpoint != "" {
		endpoint := strings.ReplaceAll(opCfg.Endpoint, "{name}", url.PathEscape(name))
		return endpoint
	}

	return defaultEndpoint
}

func (c *Client) operationMethod(op string) string {
	if opCfg := c.operationConfig(op); opCfg != nil && opCfg.Method != "" {
		return strings.ToUpper(opCfg.Method)
	}
	if op == "set" {
		return strings.ToUpper(c.cfg.Method)
	}
	if op == "delete" {
		return http.MethodDelete
	}
	return http.MethodGet
}

func (c *Client) operationStatusCodes(op string) []int {
	if opCfg := c.operationConfig(op); opCfg != nil && len(opCfg.StatusCodes) > 0 {
		return opCfg.StatusCodes
	}
	if op == "set" {
		return []int{http.StatusOK, http.StatusCreated, http.StatusNoContent}
	}
	if op == "delete" {
		return []int{http.StatusOK, http.StatusNoContent}
	}
	return []int{http.StatusOK}
}

func (c *Client) operationParser(op string) ResponseParser {
	if opCfg := c.operationConfig(op); opCfg != nil && opCfg.ResponseParser != nil {
		return NewParserFromConfig(opCfg.ResponseParser)
	}
	if op == "list" {
		return c.parser
	}
	return GetParserForVaultType(c.cfg.GetVaultType())
}

func isSuccessStatus(status int, allowed []int) bool {
	for _, code := range allowed {
		if status == code {
			return true
		}
	}
	return false
}

// GetSecret retrieves a secret from the vault
func (c *Client) GetSecret(name string) (*Secret, error) {
	vaultType := strings.ToLower(c.cfg.GetVaultType())
	if vaultType == "vaultwarden" || vaultType == "bitwarden" {
		return c.getSecretFromList(name)
	}

	return c.getSecretByOperation(name)
}

func (c *Client) getSecretFromList(name string) (*Secret, error) {
	secrets, err := c.ListSecrets()
	if err != nil {
		return nil, err
	}

	found := false
	for _, s := range secrets {
		if s == name {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("secret not found: %s", name)
	}

	url := strings.TrimSuffix(c.cfg.Endpoint, "/")
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.addAuthHeaders(req); err != nil {
		return nil, fmt.Errorf("failed to add auth headers: %w", err)
	}
	c.addCustomHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	nameField := c.cfg.FieldNames.NameField
	if nameField == "" {
		nameField = "name"
	}

	if items, ok := data["data"].([]interface{}); ok {
		for _, item := range items {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemName, ok := itemMap[nameField].(string); ok && itemName == name {
					valueField := c.cfg.FieldNames.ValueField
					if valueField == "" {
						valueField = "value"
					}

					valueData := itemMap[valueField]
					var valueStr string
					switch v := valueData.(type) {
					case map[string]interface{}, []interface{}:
						if jsonBytes, err := json.Marshal(v); err == nil {
							valueStr = string(jsonBytes)
						} else {
							valueStr = fmt.Sprintf("%v", v)
						}
					default:
						valueStr = fmt.Sprintf("%v", v)
					}

					secret := &Secret{
						Name:  name,
						Value: valueStr,
					}
					return secret, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("could not extract secret data for: %s", name)
}

func (c *Client) getSecretByOperation(name string) (*Secret, error) {
	endpoint := c.operationEndpoint("get", name)
	method := c.operationMethod("get")
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.addAuthHeaders(req); err != nil {
		return nil, fmt.Errorf("failed to add auth headers: %w", err)
	}
	c.addCustomHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	defer resp.Body.Close()

	if !isSuccessStatus(resp.StatusCode, c.operationStatusCodes("get")) {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	value, err := c.operationParser("get").ParseGetValue(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse get response: %w", err)
	}

	return &Secret{Name: name, Value: value}, nil
}

// ListSecrets lists all secrets in the vault
func (c *Client) ListSecrets() ([]string, error) {
	endpoint := c.operationEndpoint("list", "")
	method := c.operationMethod("list")
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.addAuthHeaders(req); err != nil {
		return nil, fmt.Errorf("failed to add auth headers: %w", err)
	}
	c.addCustomHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}
	defer resp.Body.Close()

	if !isSuccessStatus(resp.StatusCode, c.operationStatusCodes("list")) {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vault returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Use the configured parser to extract secret names
	names, err := c.operationParser("list").ParseList(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	return names, nil
}

// SetSecret sets a secret in the vault
func (c *Client) SetSecret(name, value string) error {
	endpoint := c.operationEndpoint("set", name)

	payload := map[string]interface{}{
		c.cfg.FieldNames.NameField: name,
	}

	// Parse value field - if it's a JSON string, unmarshal it
	valueField := c.cfg.FieldNames.ValueField
	var valueData interface{} = value

	if strings.HasPrefix(strings.TrimSpace(value), "{") || strings.HasPrefix(strings.TrimSpace(value), "[") {
		var jsonData interface{}
		if err := json.Unmarshal([]byte(value), &jsonData); err == nil {
			valueData = jsonData
		}
	}

	payload[valueField] = valueData

	// If the value field is "login" (Vaultwarden-style), add type field
	if strings.ToLower(valueField) == "login" {
		payload["type"] = 1
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	method := c.operationMethod("set")
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.addAuthHeaders(req); err != nil {
		return fmt.Errorf("failed to add auth headers: %w", err)
	}
	c.addCustomHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to set secret: %w", err)
	}
	defer resp.Body.Close()

	if !isSuccessStatus(resp.StatusCode, c.operationStatusCodes("set")) {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vault returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DeleteSecret deletes a secret from the vault
func (c *Client) DeleteSecret(name string) error {
	endpoint := c.operationEndpoint("delete", name)
	method := c.operationMethod("delete")
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.addAuthHeaders(req); err != nil {
		return fmt.Errorf("failed to add auth headers: %w", err)
	}
	c.addCustomHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}
	defer resp.Body.Close()

	if !isSuccessStatus(resp.StatusCode, c.operationStatusCodes("delete")) {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vault returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// TestConnection tests if the connection to the vault works
func (c *Client) TestConnection() error {
	url := strings.TrimSuffix(c.cfg.Endpoint, "/")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.addAuthHeaders(req); err != nil {
		return fmt.Errorf("failed to add auth headers: %w", err)
	}
	c.addCustomHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("vault returned unexpected status %d", resp.StatusCode)
	}

	return nil
}

// getOAuthToken exchanges OAuth 2.0 credentials for an access token
func (c *Client) getOAuthToken() (string, error) {
	// Return cached token if still valid
	if c.oauthToken != "" && time.Now().Before(c.oauthExpires) {
		return c.oauthToken, nil
	}

	if c.cfg.Auth == nil || c.cfg.Auth.OAuth == nil {
		return "", fmt.Errorf("OAuth config not found")
	}

	oauthCfg := c.cfg.Auth.OAuth

	if oauthCfg.ClientID == "" || oauthCfg.ClientSecret == "" {
		return "", fmt.Errorf("client_id or client_secret not configured for OAuth 2.0")
	}

	scope := oauthCfg.Scope
	if scope == "" {
		scope = "api"
	}

	// Get token endpoint from config or use smart default
	tokenURL := oauthCfg.TokenEndpoint
	if tokenURL == "" {
		tokenURL = c.getDefaultTokenEndpoint()
	}

	// Build request parameters
	params := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     oauthCfg.ClientID,
		"client_secret": oauthCfg.ClientSecret,
		"scope":         scope,
	}

	// Add extra OAuth parameters from config (device ID, etc.)
	if oauthCfg.ExtraParams != nil {
		for k, v := range oauthCfg.ExtraParams {
			params[k] = v
		}
	}

	// Encode parameters
	data := encodeParams(params)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create OAuth token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Note: We intentionally do NOT call addCustomHeaders() here because the token endpoint
	// needs specific headers and should not inherit the vault's custom headers

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to exchange OAuth credentials (URL: %s): %w", tokenURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read OAuth response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OAuth token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp map[string]interface{}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse OAuth response: %w", err)
	}

	token, ok := tokenResp["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("access_token not found in OAuth response: %v", tokenResp)
	}

	expiresIn, ok := tokenResp["expires_in"].(float64)
	if !ok {
		expiresIn = 3600 // Default to 1 hour
	}

	// Cache the token with a 5-minute buffer before expiry
	c.oauthToken = token
	c.oauthExpires = time.Now().Add(time.Duration(expiresIn-300) * time.Second)

	return token, nil
}

// getDefaultTokenEndpoint returns the default OAuth token endpoint for the vault type
func (c *Client) getDefaultTokenEndpoint() string {
	vaultType := c.cfg.GetVaultType()

	switch strings.ToLower(vaultType) {
	case "vaultwarden":
		// Vaultwarden: strip /api/ciphers and use /identity/connect/token
		baseURL := strings.TrimSuffix(c.cfg.Endpoint, "/api/ciphers")
		if baseURL == c.cfg.Endpoint {
			// If endpoint doesn't end with /api/ciphers, assume it's the base URL
			baseURL = strings.TrimSuffix(c.cfg.Endpoint, "/")
		}
		return fmt.Sprintf("%s/identity/connect/token", baseURL)

	case "bitwarden":
		endpoint := strings.TrimSuffix(c.cfg.Endpoint, "/")
		if strings.Contains(endpoint, "api.bitwarden.com") {
			return "https://identity.bitwarden.com/connect/token"
		}
		baseURL := strings.TrimSuffix(endpoint, "/api/ciphers")
		if baseURL == endpoint {
			baseURL = endpoint
		}
		return fmt.Sprintf("%s/identity/connect/token", baseURL)

	case "vault":
		// HashiCorp Vault: /v1/auth/oauth/token
		baseURL := strings.TrimSuffix(c.cfg.Endpoint, "/v1")
		if baseURL == c.cfg.Endpoint {
			baseURL = strings.TrimSuffix(c.cfg.Endpoint, "/")
		}
		return fmt.Sprintf("%s/v1/auth/oauth/token", baseURL)

	case "azure":
		// Azure AD: {tenant}/oauth2/v2.0/token
		// User should provide full endpoint in config
		return "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token"

	default:
		// Generic: use endpoint + /oauth/token
		return fmt.Sprintf("%s/oauth/token", strings.TrimSuffix(c.cfg.Endpoint, "/"))
	}
}

// encodeParams encodes a map of parameters as application/x-www-form-urlencoded
func encodeParams(params map[string]string) string {
	var pairs []string
	for k, v := range params {
		pairs = append(pairs, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(v)))
	}
	sort.Strings(pairs) // For consistency
	return strings.Join(pairs, "&")
}

// addAuthHeaders adds authentication headers based on the configured auth method
// Returns an error if authentication setup fails
func (c *Client) addAuthHeaders(req *http.Request) error {
	authMethod := c.cfg.Auth.Method
	authHeaders := c.cfg.Auth.Headers

	switch strings.ToLower(authMethod) {
	case "oauth2":
		token, err := c.getOAuthToken()
		if err != nil {
			return fmt.Errorf("failed to get OAuth token: %w", err)
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	case "bearer":
		if token, ok := authHeaders["token"]; ok {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		}
	case "basic":
		if username, ok := authHeaders["username"]; ok {
			if password, ok := authHeaders["password"]; ok {
				auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
				req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
			}
		}
	case "api_key":
		if key, ok := authHeaders["api_key"]; ok {
			req.Header.Set("X-API-Key", key)
		}
	case "custom":
		// Add all custom headers as-is
		for k, v := range authHeaders {
			req.Header.Set(k, v)
		}
	}
	return nil
}

// addCustomHeaders adds custom headers from config
func (c *Client) addCustomHeaders(req *http.Request) {
	for k, v := range c.cfg.Headers {
		req.Header.Set(k, v)
	}
}
