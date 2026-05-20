package vault

import (
	"encoding/json"
	"os"
	"runtime"
	"testing"

	"github.com/pacorreia/vaults-syncer/config"
)

// echoCmd returns the platform-appropriate command to echo a string to stdout.
// On Windows "cmd /C echo" is used; everywhere else plain "echo".
func echoCmd(text string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/C", "echo", text}
	}
	return "echo", []string{text}
}

// shellCmd runs a shell one-liner (sh -c "…").
func shellCmd(script string) (string, []string) {
	return "sh", []string{"-c", script}
}

// makeToolCfg is a test helper that builds a minimal ExternalToolConfig.
func makeToolCfg(ops map[string]*config.ToolOperationConfig) *config.ExternalToolConfig {
	return &config.ExternalToolConfig{Operations: ops}
}

// makeVaultCfg returns a VaultConfig pre-populated for tool tests.
func makeVaultCfg(id string, toolCfg *config.ExternalToolConfig) *config.VaultConfig {
	return &config.VaultConfig{
		ID:           id,
		Type:         "tool",
		Timeout:      5,
		ResolvedTool: toolCfg,
	}
}

// ---------------------------------------------------------------------------
// ToolBackend - basic construction
// ---------------------------------------------------------------------------

func TestNewToolBackend(t *testing.T) {
	toolCfg := makeToolCfg(nil)
	vcfg := makeVaultCfg("tb", toolCfg)
	b := NewToolBackend(vcfg, toolCfg)

	if b == nil {
		t.Fatal("expected non-nil ToolBackend")
	}
	if b.Type() != "tool" {
		t.Errorf("expected Type() == 'tool', got %q", b.Type())
	}
}

// ---------------------------------------------------------------------------
// Capabilities
// ---------------------------------------------------------------------------

func TestToolBackendCapabilities(t *testing.T) {
	cmd, args := echoCmd("[]")

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"list":   {Command: cmd, Args: args},
		"get":    {Command: cmd, Args: args},
		"set":    {Command: cmd, Args: args},
		"delete": {Command: cmd, Args: args},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)
	caps := b.Capabilities()

	if !caps.CanList || !caps.CanGet || !caps.CanSet || !caps.CanDelete || !caps.CanSync {
		t.Errorf("expected all capabilities true when all ops defined, got %+v", caps)
	}
}

func TestToolBackendCapabilitiesPartial(t *testing.T) {
	cmd, args := echoCmd("[]")
	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"list": {Command: cmd, Args: args},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)
	caps := b.Capabilities()

	if !caps.CanList {
		t.Error("expected CanList true")
	}
	if caps.CanGet || caps.CanSet || caps.CanDelete {
		t.Error("expected CanGet, CanSet, CanDelete false when ops not defined")
	}
}

// ---------------------------------------------------------------------------
// ListSecrets - JSON format
// ---------------------------------------------------------------------------

func TestToolBackendListSecretsJSON(t *testing.T) {
	payload, _ := json.Marshal(map[string]interface{}{
		"secrets": []map[string]string{
			{"Name": "foo"},
			{"Name": "bar"},
		},
	})
	cmd, args := echoCmd(string(payload))

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"list": {
			Command: cmd,
			Args:    args,
			Output:  config.ToolOutputConfig{Format: "json", Path: "secrets", NameField: "Name"},
		},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	names, err := b.ListSecrets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d: %v", len(names), names)
	}
	if names[0] != "foo" || names[1] != "bar" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestToolBackendListSecretsLines(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("newline echo differs on Windows")
	}

	cmd, args := shellCmd("printf 'secret-a\\nsecret-b\\n'")

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"list": {
			Command: cmd,
			Args:    args,
			Output:  config.ToolOutputConfig{Format: "lines"},
		},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	names, err := b.ListSecrets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 || names[0] != "secret-a" || names[1] != "secret-b" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestToolBackendListSecretsMissingOp(t *testing.T) {
	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	_, err := b.ListSecrets()
	if err == nil {
		t.Fatal("expected error for missing list op")
	}
}

// ---------------------------------------------------------------------------
// GetSecret
// ---------------------------------------------------------------------------

func TestToolBackendGetSecretJSON(t *testing.T) {
	payload := `{"SecretString": "my-value"}`
	cmd, args := echoCmd(payload)

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"get": {
			Command: cmd,
			Args:    args,
			Output:  config.ToolOutputConfig{Format: "json", Path: "SecretString"},
		},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	secret, err := b.GetSecret("my-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret.Name != "my-secret" {
		t.Errorf("expected name 'my-secret', got %q", secret.Name)
	}
	if secret.Value != "my-value" {
		t.Errorf("expected value 'my-value', got %q", secret.Value)
	}
}

func TestToolBackendGetSecretText(t *testing.T) {
	cmd, args := echoCmd("plain-text-value")

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"get": {
			Command: cmd,
			Args:    args,
			Output:  config.ToolOutputConfig{Format: "text"},
		},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	secret, err := b.GetSecret("s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret.Value != "plain-text-value" {
		t.Errorf("expected 'plain-text-value', got %q", secret.Value)
	}
}

// ---------------------------------------------------------------------------
// SetSecret & DeleteSecret - verify template rendering
// ---------------------------------------------------------------------------

func TestToolBackendSetSecret(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell quoting differs on Windows")
	}

	// Write rendered args to a temp file so we can inspect them.
	tmp, err := os.CreateTemp("", "tool-set-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	script := "echo -n \"$1=$2\" > " + tmp.Name()

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"set": {
			Command: "sh",
			Args:    []string{"-c", script, "--", "{{.Name}}", "{{.Value}}"},
		},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	if err := b.SetSecret("KEY", "VALUE"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(tmp.Name())
	if string(got) != "KEY=VALUE" {
		t.Errorf("expected 'KEY=VALUE' in file, got %q", string(got))
	}
}

func TestToolBackendDeleteSecret(t *testing.T) {
	cmd, args := echoCmd("deleted")

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"delete": {
			Command: cmd,
			Args:    append(args, "{{.Name}}"),
		},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	if err := b.DeleteSecret("my-secret"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestConnection
// ---------------------------------------------------------------------------

func TestToolBackendTestConnectionUsesTestOp(t *testing.T) {
	cmd, args := echoCmd("ok")

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"test": {Command: cmd, Args: args},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	if err := b.TestConnection(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolBackendTestConnectionFallsBackToList(t *testing.T) {
	cmd, args := echoCmd(`{"items":[]}`)

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"list": {
			Command: cmd,
			Args:    args,
			Output:  config.ToolOutputConfig{Format: "json", Path: "items"},
		},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	if err := b.TestConnection(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolBackendTestConnectionNoOps(t *testing.T) {
	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	if err := b.TestConnection(); err == nil {
		t.Fatal("expected error when no ops defined")
	}
}

// ---------------------------------------------------------------------------
// Error handling - non-zero exit code
// ---------------------------------------------------------------------------

func TestToolBackendCommandFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exit code handling differs on Windows")
	}

	toolCfg := makeToolCfg(map[string]*config.ToolOperationConfig{
		"list": {
			Command: "sh",
			Args:    []string{"-c", "exit 1"},
			Output:  config.ToolOutputConfig{Format: "json", Path: "items"},
		},
	})
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	_, err := b.ListSecrets()
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
}

// ---------------------------------------------------------------------------
// Environment variable injection
// ---------------------------------------------------------------------------

func TestToolBackendEnvInjection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("env var echo differs on Windows")
	}

	tmp, err := os.CreateTemp("", "tool-env-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	toolCfg := &config.ExternalToolConfig{
		Env: map[string]string{"TOOL_TEST_VAR": "injected"},
		Operations: map[string]*config.ToolOperationConfig{
			"list": {
				Command: "sh",
				Args:    []string{"-c", "echo -n $TOOL_TEST_VAR > " + tmp.Name()},
				Output:  config.ToolOutputConfig{Format: "lines"},
			},
		},
	}
	b := NewToolBackend(makeVaultCfg("tb", toolCfg), toolCfg)

	_, err = b.ListSecrets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(tmp.Name())
	if string(got) != "injected" {
		t.Errorf("expected env var to be injected, file contains %q", string(got))
	}
}

// ---------------------------------------------------------------------------
// Unit tests for internal helpers
// ---------------------------------------------------------------------------

func TestRenderArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		data    templateData
		want    []string
		wantErr bool
	}{
		{
			name: "no templates",
			args: []string{"secretsmanager", "list-secrets"},
			data: templateData{},
			want: []string{"secretsmanager", "list-secrets"},
		},
		{
			name: "Name template",
			args: []string{"get-secret-value", "--secret-id", "{{.Name}}"},
			data: templateData{Name: "my-secret"},
			want: []string{"get-secret-value", "--secret-id", "my-secret"},
		},
		{
			name: "Name and Value templates",
			args: []string{"put-secret-value", "--secret-id", "{{.Name}}", "--secret-string", "{{.Value}}"},
			data: templateData{Name: "KEY", Value: "VAL"},
			want: []string{"put-secret-value", "--secret-id", "KEY", "--secret-string", "VAL"},
		},
		{
			name:    "invalid template",
			args:    []string{"{{.Undefined"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderArgs(tt.args, tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("arg[%d]: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestParseLines(t *testing.T) {
	input := []byte("alpha\nbeta\n\ngamma\n")
	got := parseLines(input)
	if len(got) != 3 || got[0] != "alpha" || got[1] != "beta" || got[2] != "gamma" {
		t.Errorf("unexpected result: %v", got)
	}
}

func TestParseJSONList(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"keys": []map[string]string{
			{"id": "k1"},
			{"id": "k2"},
		},
	})

	names, err := parseJSONList(data, "keys", "id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 || names[0] != "k1" || names[1] != "k2" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestParseJSONListStringItems(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"names": []string{"a", "b"},
	})

	names, err := parseJSONList(data, "names", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("unexpected names: %v", names)
	}
}

func TestParseJSONValue(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": "the-value",
		},
	})

	val, err := parseJSONValue(data, "outer.inner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "the-value" {
		t.Errorf("expected 'the-value', got %q", val)
	}
}

func TestJsonNavigate(t *testing.T) {
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "result",
		},
	}

	val, err := jsonNavigate(data, "a.b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "result" {
		t.Errorf("expected 'result', got %v", val)
	}
}

func TestJsonNavigateEmptyPath(t *testing.T) {
	data := map[string]interface{}{"x": 1}
	val, err := jsonNavigate(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if val == nil {
		t.Fatal("expected non-nil")
	}
}

func TestJsonNavigateMissingKey(t *testing.T) {
	data := map[string]interface{}{"a": 1}
	_, err := jsonNavigate(data, "missing")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}
