package vault

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pacorreia/vaults-syncer/config"
)

// ResponseParser defines the interface for extracting data from vault responses
type ResponseParser interface {
	ParseList(body []byte) ([]string, error)
	ParseGetValue(body []byte) (string, error)
}

// JsonPathParser extracts fields from responses using simple JSONPath-like notation
type JsonPathParser struct {
	ListPath   string // Path to array of items (e.g., "data" or "value.items")
	NameField  string // Field containing secret name (e.g., "name" or "id")
	ValuePath  string // Path to value field (e.g., "value" or "data.value")
	ValueField string // Simple field name for value (alternative to ValuePath)
}

// ParseList extracts a list of secret names from a response
func (p *JsonPathParser) ParseList(body []byte) ([]string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Navigate to list path
	var items []interface{}
	if p.ListPath != "" {
		val, err := p.getValueAtPath(data, p.ListPath)
		if err == nil {
			if itemsArr, ok := val.([]interface{}); ok {
				items = itemsArr
			}
		}
	}

	// If no items found, try common defaults
	if len(items) == 0 {
		if val, ok := data["data"].([]interface{}); ok {
			items = val
		} else if val, ok := data["value"].([]interface{}); ok {
			items = val
		} else if val, ok := data["keys"].([]interface{}); ok {
			items = val
		}
	}

	var names []string
	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			nameField := p.NameField
			if nameField == "" {
				nameField = "name"
			}

			if name, ok := itemMap[nameField].(string); ok {
				names = append(names, name)
			}
		} else {
			// If item is a string directly, use it as name
			if str, ok := item.(string); ok {
				names = append(names, str)
			}
		}
	}

	return names, nil
}

// ParseGetValue extracts a single value from a response
func (p *JsonPathParser) ParseGetValue(body []byte) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Try to extract value using ValuePath
	if p.ValuePath != "" {
		val, err := p.getValueAtPath(data, p.ValuePath)
		if err == nil {
			return p.valueToString(val), nil
		}
	}

	// Fall back to simple field name
	if p.ValueField != "" {
		if val, ok := data[p.ValueField]; ok {
			return p.valueToString(val), nil
		}
	}

	// Try common defaults
	for _, fieldName := range []string{"value", "data", "secret"} {
		if val, ok := data[fieldName]; ok {
			return p.valueToString(val), nil
		}
	}

	return "", fmt.Errorf("could not extract value from response")
}

// getValueAtPath navigates through nested JSON using dot notation
func (p *JsonPathParser) getValueAtPath(data interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil, fmt.Errorf("path not found: %s", part)
			}
		default:
			return nil, fmt.Errorf("cannot navigate through %T", v)
		}
	}

	return current, nil
}

// valueToString converts various types to string
func (p *JsonPathParser) valueToString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case map[string]interface{}, []interface{}:
		// Complex objects are serialized as JSON
		if jsonBytes, err := json.Marshal(v); err == nil {
			return string(jsonBytes)
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// GetParserForVaultType returns an appropriate parser based on vault type
func GetParserForVaultType(vaultType string) ResponseParser {
	switch strings.ToLower(vaultType) {
	case "vaultwarden":
		return &JsonPathParser{
			ListPath:  "data",
			NameField: "name",
			ValuePath: "data",
		}

	case "bitwarden":
		return &JsonPathParser{
			ListPath:  "data",
			NameField: "name",
			ValuePath: "data",
		}

	case "vault":
		return &JsonPathParser{
			ListPath:  "data.keys",
			NameField: "key",
			ValuePath: "data.data",
		}

	case "azure":
		return &JsonPathParser{
			ListPath:  "value",
			NameField: "name",
			ValuePath: "value",
		}

	case "keeper":
		return &JsonPathParser{
			ListPath:  "records",
			NameField: "title",
			ValuePath: "data",
		}

	case "aws":
		return &JsonPathParser{
			ListPath:  "SecretList",
			NameField: "Name",
			ValuePath: "SecretString",
		}

	default:
		// Generic parser with smart defaults
		return &JsonPathParser{
			ListPath:  "data",
			NameField: "name",
			ValuePath: "value",
		}
	}
}

// NewParserFromConfig creates a parser from configuration
func NewParserFromConfig(cfg *config.ResponseParserConfig) ResponseParser {
	if cfg == nil {
		// Use generic defaults
		return GetParserForVaultType("generic")
	}

	return &JsonPathParser{
		ListPath:  cfg.ListPath,
		NameField: cfg.NameField,
		ValuePath: cfg.ValuePath,
	}
}
