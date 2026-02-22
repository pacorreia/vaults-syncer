package vault

import (
	"strings"
	"testing"

	"github.com/pacorreia/vaults-syncer/config"
)

func TestJsonPathParser_ParseList(t *testing.T) {
	tests := []struct {
		name    string
		parser  *JsonPathParser
		body    string
		want    []string
		wantErr bool
	}{
		{
			name: "simple list with data key",
			parser: &JsonPathParser{
				ListPath:  "data",
				NameField: "name",
			},
			body: `{
				"data": [
					{"name": "secret1"},
					{"name": "secret2"},
					{"name": "secret3"}
				]
			}`,
			want:    []string{"secret1", "secret2", "secret3"},
			wantErr: false,
		},
		{
			name: "list with value key",
			parser: &JsonPathParser{
				ListPath:  "value",
				NameField: "id",
			},
			body: `{
				"value": [
					{"id": "key1", "attributes": {}},
					{"id": "key2", "attributes": {}},
					{"id": "key3", "attributes": {}}
				]
			}`,
			want:    []string{"key1", "key2", "key3"},
			wantErr: false,
		},
		{
			name: "nested list path",
			parser: &JsonPathParser{
				ListPath:  "response.items",
				NameField: "secretName",
			},
			body: `{
				"response": {
					"items": [
						{"secretName": "api-key"},
						{"secretName": "db-password"}
					]
				}
			}`,
			want:    []string{"api-key", "db-password"},
			wantErr: false,
		},
		{
			name: "default data key fallback",
			parser: &JsonPathParser{
				ListPath:  "",
				NameField: "name",
			},
			body: `{
				"data": [
					{"name": "default1"},
					{"name": "default2"}
				]
			}`,
			want:    []string{"default1", "default2"},
			wantErr: false,
		},
		{
			name: "array of strings",
			parser: &JsonPathParser{
				ListPath:  "keys",
				NameField: "",
			},
			body: `{
				"keys": ["secret-1", "secret-2", "secret-3"]
			}`,
			want:    []string{"secret-1", "secret-2", "secret-3"},
			wantErr: false,
		},
		{
			name: "invalid json",
			parser: &JsonPathParser{
				ListPath:  "data",
				NameField: "name",
			},
			body:    `{"data": [invalid json}`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty list",
			parser: &JsonPathParser{
				ListPath:  "data",
				NameField: "name",
			},
			body:    `{"data": []}`,
			want:    []string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.parser.ParseList([]byte(tt.body))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %d items, want %d", len(got), len(tt.want))
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("item %d: got %s, want %s", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestJsonPathParser_ParseGetValue(t *testing.T) {
	tests := []struct {
		name    string
		parser  *JsonPathParser
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "simple value field",
			parser: &JsonPathParser{
				ValueField: "value",
			},
			body: `{
				"name": "my-secret",
				"value": "secret-password-123"
			}`,
			want:    "secret-password-123",
			wantErr: false,
		},
		{
			name: "nested value path",
			parser: &JsonPathParser{
				ValuePath: "data.value",
			},
			body: `{
				"data": {
					"value": "nested-secret-value",
					"metadata": {}
				}
			}`,
			want:    "nested-secret-value",
			wantErr: false,
		},
		{
			name: "azure key vault format",
			parser: &JsonPathParser{
				ValuePath: "value",
			},
			body: `{
				"value": "azure-secret-123",
				"id": "https://vault.azure.net/secrets/my-secret/version",
				"attributes": {
					"enabled": true
				}
			}`,
			want:    "azure-secret-123",
			wantErr: false,
		},
		{
			name: "vaultwarden format",
			parser: &JsonPathParser{
				ValuePath: "data.password",
			},
			body: `{
				"data": {
					"name": "My Login",
					"login": {
						"username": "user",
						"password": "pass123"
					},
					"password": "pass123"
				}
			}`,
			want:    "pass123",
			wantErr: false,
		},
		{
			name: "default value field",
			parser: &JsonPathParser{
				ValueField: "",
			},
			body: `{
				"value": "default-value"
			}`,
			want:    "default-value",
			wantErr: false,
		},
		{
			name: "invalid json",
			parser: &JsonPathParser{
				ValueField: "value",
			},
			body:    `{"value": invalid}`,
			want:    "",
			wantErr: true,
		},
		{
			name: "missing value field",
			parser: &JsonPathParser{
				ValueField: "nonexistent",
			},
			body:    `{"other": "some-value", "field": "another"}`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.parser.ParseGetValue([]byte(tt.body))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetValueAtPath(t *testing.T) {
	parser := &JsonPathParser{}

	tests := []struct {
		name    string
		data    map[string]interface{}
		path    string
		want    interface{}
		wantErr bool
	}{
		{
			name: "single level",
			data: map[string]interface{}{
				"key": "value",
			},
			path:    "key",
			want:    "value",
			wantErr: false,
		},
		{
			name: "nested path",
			data: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": "nested-value",
				},
			},
			path:    "level1.level2",
			want:    "nested-value",
			wantErr: false,
		},
		{
			name: "deeply nested",
			data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "deep-value",
					},
				},
			},
			path:    "a.b.c",
			want:    "deep-value",
			wantErr: false,
		},
		{
			name: "path not found",
			data: map[string]interface{}{
				"key": "value",
			},
			path:    "nonexistent",
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty path",
			data: map[string]interface{}{
				"key": "value",
			},
			path:    "",
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.getValueAtPath(tt.data, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.want != nil && got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetParserForVaultType(t *testing.T) {
	tests := []struct {
		name          string
		vaultType     string
		wantListPath  string
		wantNameField string
		wantValuePath string
	}{
		{
			name:          "vaultwarden",
			vaultType:     "vaultwarden",
			wantListPath:  "data",
			wantNameField: "name",
			wantValuePath: "data",
		},
		{
			name:          "vault",
			vaultType:     "vault",
			wantListPath:  "data.keys",
			wantNameField: "key",
			wantValuePath: "data.data",
		},
		{
			name:          "azure",
			vaultType:     "azure",
			wantListPath:  "value",
			wantNameField: "name",
			wantValuePath: "value",
		},
		{
			name:          "aws",
			vaultType:     "aws",
			wantListPath:  "SecretList",
			wantNameField: "Name",
			wantValuePath: "SecretString",
		},
		{
			name:          "generic default",
			vaultType:     "generic",
			wantListPath:  "data",
			wantNameField: "name",
			wantValuePath: "value",
		},
		{
			name:          "unknown defaults to generic",
			vaultType:     "unknown",
			wantListPath:  "data",
			wantNameField: "name",
			wantValuePath: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := GetParserForVaultType(tt.vaultType)

			if parser == nil {
				t.Fatal("expected parser, got nil")
			}

			jsonParser, ok := parser.(*JsonPathParser)
			if !ok {
				t.Fatal("expected JsonPathParser type")
			}

			if jsonParser.ListPath != tt.wantListPath {
				t.Errorf("ListPath: got %s, want %s", jsonParser.ListPath, tt.wantListPath)
			}
			if jsonParser.NameField != tt.wantNameField {
				t.Errorf("NameField: got %s, want %s", jsonParser.NameField, tt.wantNameField)
			}
			if jsonParser.ValuePath != tt.wantValuePath {
				t.Errorf("ValuePath: got %s, want %s", jsonParser.ValuePath, tt.wantValuePath)
			}
		})
	}
}

func TestNewParserFromConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.ResponseParserConfig
		wantListPath  string
		wantNameField string
		wantValuePath string
	}{
		{
			name: "custom parser config",
			config: &config.ResponseParserConfig{
				ListPath:  "custom.path",
				NameField: "customName",
				ValuePath: "custom.value",
			},
			wantListPath:  "custom.path",
			wantNameField: "customName",
			wantValuePath: "custom.value",
		},
		{
			name:          "nil config uses generic defaults",
			config:        nil,
			wantListPath:  "data",
			wantNameField: "name",
			wantValuePath: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParserFromConfig(tt.config)

			if parser == nil {
				t.Fatal("expected parser, got nil")
			}

			jsonParser, ok := parser.(*JsonPathParser)
			if !ok {
				t.Fatal("expected JsonPathParser type")
			}

			if jsonParser.ListPath != tt.wantListPath {
				t.Errorf("ListPath: got %s, want %s", jsonParser.ListPath, tt.wantListPath)
			}
			if jsonParser.NameField != tt.wantNameField {
				t.Errorf("NameField: got %s, want %s", jsonParser.NameField, tt.wantNameField)
			}
			if jsonParser.ValuePath != tt.wantValuePath {
				t.Errorf("ValuePath: got %s, want %s", jsonParser.ValuePath, tt.wantValuePath)
			}
		})
	}
}

func TestParseListWithStringItems(t *testing.T) {
	parser := &JsonPathParser{
		ListPath:  "data",
		NameField: "name",
	}

	// Test when items are strings directly instead of objects
	body := []byte(`{"data": ["secret1", "secret2", "secret3"]}`)

	names, err := parser.ParseList(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}

	expected := []string{"secret1", "secret2", "secret3"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("name[%d]: got %s, want %s", i, name, expected[i])
		}
	}
}

func TestParseListWithDefaultPaths(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []string
	}{
		{
			name:     "data array",
			body:     `{"data": [{"name": "s1"}, {"name": "s2"}]}`,
			expected: []string{"s1", "s2"},
		},
		{
			name:     "value array",
			body:     `{"value": [{"name": "s1"}, {"name": "s2"}]}`,
			expected: []string{"s1", "s2"},
		},
		{
			name:     "keys array",
			body:     `{"keys": [{"name": "s1"}, {"name": "s2"}]}`,
			expected: []string{"s1", "s2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := &JsonPathParser{} // Empty parser uses defaults

			names, err := parser.ParseList([]byte(tt.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(names) != len(tt.expected) {
				t.Errorf("expected %d names, got %d", len(tt.expected), len(names))
			}

			for i, name := range names {
				if name != tt.expected[i] {
					t.Errorf("name[%d]: got %s, want %s", i, name, tt.expected[i])
				}
			}
		})
	}
}

func TestValueToStringArray(t *testing.T) {
	parser := &JsonPathParser{}

	// Test array values
	arrayValue := []interface{}{"item1", "item2"}
	result := parser.valueToString(arrayValue)

	if !strings.Contains(result, "item1") || !strings.Contains(result, "item2") {
		t.Errorf("expected JSON array string, got %s", result)
	}
}

func TestValueToStringNil(t *testing.T) {
	parser := &JsonPathParser{}

	result := parser.valueToString(nil)
	if result != "<nil>" {
		t.Errorf("expected <nil> for nil, got %s", result)
	}
}

func TestValueToStringMarshalError(t *testing.T) {
	parser := &JsonPathParser{}

	value := map[string]interface{}{"bad": make(chan int)}
	result := parser.valueToString(value)
	if !strings.Contains(result, "bad") {
		t.Errorf("expected fallback string to contain key, got %s", result)
	}
}
