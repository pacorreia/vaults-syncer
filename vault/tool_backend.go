package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/pacorreia/vaults-syncer/config"
)

// templateData holds the variables available inside tool argument templates.
type templateData struct {
	Name  string
	Value string
}

// ToolBackend implements the Backend interface by running external CLI commands.
// Each vault operation (list, get, set, delete, test) is driven by the command
// definition in the associated ExternalToolConfig.
type ToolBackend struct {
	cfg     *config.VaultConfig
	toolCfg *config.ExternalToolConfig
}

// NewToolBackend creates a ToolBackend for the given vault and resolved tool config.
func NewToolBackend(cfg *config.VaultConfig, toolCfg *config.ExternalToolConfig) *ToolBackend {
	return &ToolBackend{cfg: cfg, toolCfg: toolCfg}
}

// Type implements Backend.Type.
func (b *ToolBackend) Type() string {
	return "tool"
}

// Capabilities implements Backend.Capabilities.
// The tool backend advertises capabilities based on which operations are defined.
func (b *ToolBackend) Capabilities() BackendCapabilities {
	ops := b.toolCfg.Operations
	return BackendCapabilities{
		CanList:   ops["list"] != nil,
		CanGet:    ops["get"] != nil,
		CanSet:    ops["set"] != nil,
		CanDelete: ops["delete"] != nil,
		CanSync:   ops["list"] != nil && ops["get"] != nil,
	}
}

// ListSecrets implements Backend.ListSecrets by running the "list" operation.
func (b *ToolBackend) ListSecrets() ([]string, error) {
	op, err := b.requireOp("list")
	if err != nil {
		return nil, err
	}

	stdout, err := b.runOp(op, templateData{})
	if err != nil {
		return nil, fmt.Errorf("list operation failed: %w", err)
	}

	return b.parseListOutput(op, stdout)
}

// GetSecret implements Backend.GetSecret by running the "get" operation.
func (b *ToolBackend) GetSecret(name string) (*Secret, error) {
	op, err := b.requireOp("get")
	if err != nil {
		return nil, err
	}

	stdout, err := b.runOp(op, templateData{Name: name})
	if err != nil {
		return nil, fmt.Errorf("get operation failed for secret %q: %w", name, err)
	}

	value, err := b.parseGetOutput(op, stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse get output for secret %q: %w", name, err)
	}

	return &Secret{Name: name, Value: value}, nil
}

// SetSecret implements Backend.SetSecret by running the "set" operation.
func (b *ToolBackend) SetSecret(name, value string) error {
	op, err := b.requireOp("set")
	if err != nil {
		return err
	}

	_, err = b.runOp(op, templateData{Name: name, Value: value})
	if err != nil {
		return fmt.Errorf("set operation failed for secret %q: %w", name, err)
	}
	return nil
}

// DeleteSecret implements Backend.DeleteSecret by running the "delete" operation.
func (b *ToolBackend) DeleteSecret(name string) error {
	op, err := b.requireOp("delete")
	if err != nil {
		return err
	}

	_, err = b.runOp(op, templateData{Name: name})
	if err != nil {
		return fmt.Errorf("delete operation failed for secret %q: %w", name, err)
	}
	return nil
}

// TestConnection implements Backend.TestConnection. It runs the "test" operation if
// defined; otherwise it falls back to a lightweight "list" call.
func (b *ToolBackend) TestConnection() error {
	op := b.toolCfg.Operations["test"]
	if op == nil {
		op = b.toolCfg.Operations["list"]
	}
	if op == nil {
		return fmt.Errorf("tool backend has no 'test' or 'list' operation defined")
	}

	_, err := b.runOp(op, templateData{})
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	return nil
}

// requireOp returns the operation config for the given name, or an error if absent.
func (b *ToolBackend) requireOp(name string) (*config.ToolOperationConfig, error) {
	op := b.toolCfg.Operations[name]
	if op == nil {
		return nil, fmt.Errorf("tool backend for vault %q does not define a '%s' operation", b.cfg.ID, name)
	}
	return op, nil
}

// runOp executes a ToolOperationConfig, rendering Go templates in args, injecting
// the tool-level environment variables, and returning stdout on success.
func (b *ToolBackend) runOp(op *config.ToolOperationConfig, data templateData) ([]byte, error) {
	args, err := renderArgs(op.Args, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render command args: %w", err)
	}

	timeout := time.Duration(b.cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, op.Command, args...) //nolint:gosec // command comes from operator-controlled config

	// Inject tool-level env vars and passthrough vars on top of the current process environment.
	// Build a de-duplicated env by parsing the inherited environment into a map, then applying
	// overrides so that Env and EnvPassthrough values always win over any inherited values.
	if len(b.toolCfg.Env) > 0 || len(b.toolCfg.EnvPassthrough) > 0 {
		envMap := make(map[string]string)
		for _, entry := range cmd.Environ() {
			k, v, found := strings.Cut(entry, "=")
			if !found {
				continue
			}
			envMap[k] = v
		}
		for k, v := range b.toolCfg.Env {
			envMap[k] = v
		}
		// EnvPassthrough forwards named vars from the current runtime environment.
		// Values are read at execution time, so rotated credentials are picked up
		// without restarting the daemon.
		for _, name := range b.toolCfg.EnvPassthrough {
			if v, ok := os.LookupEnv(name); ok {
				envMap[name] = v
			}
		}
		env := make([]string, 0, len(envMap))
		for k, v := range envMap {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("command execution error: %w", runErr)
		}
	}

	successCodes := op.SuccessExitCodes
	if len(successCodes) == 0 {
		successCodes = []int{0}
	}
	// isSuccessStatus is defined in client.go (same package).
	if !isSuccessStatus(exitCode, successCodes) {
		return nil, fmt.Errorf("command exited with code %d: %s", exitCode, strings.TrimSpace(stderr.String()))
	}

	return stdout.Bytes(), nil
}

// parseListOutput extracts secret names from the stdout of a list command.
func (b *ToolBackend) parseListOutput(op *config.ToolOperationConfig, stdout []byte) ([]string, error) {
	format := strings.ToLower(op.Output.Format)
	if format == "" {
		format = "json"
	}

	switch format {
	case "lines":
		return parseLines(stdout), nil
	case "json":
		return parseJSONList(stdout, op.Output.Path, op.Output.NameField)
	default:
		return nil, fmt.Errorf("unsupported output format %q for list operation", format)
	}
}

// parseGetOutput extracts a secret value from the stdout of a get command.
func (b *ToolBackend) parseGetOutput(op *config.ToolOperationConfig, stdout []byte) (string, error) {
	format := strings.ToLower(op.Output.Format)
	if format == "" {
		format = "json"
	}

	switch format {
	case "text", "lines":
		return strings.TrimSpace(string(stdout)), nil
	case "json":
		return parseJSONValue(stdout, op.Output.Path)
	default:
		return "", fmt.Errorf("unsupported output format %q for get operation", format)
	}
}

// renderArgs processes Go templates in each argument string.
// Templates are parsed with missingkey=error so that references to undefined
// fields (e.g. typos like {{.name}}) fail fast rather than silently rendering
// as "<no value>".
func renderArgs(args []string, data templateData) ([]string, error) {
	out := make([]string, len(args))
	for i, arg := range args {
		tmpl, err := template.New("arg").Option("missingkey=error").Parse(arg)
		if err != nil {
			return nil, fmt.Errorf("invalid template in arg %q: %w", arg, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("failed to execute template in arg %q: %w", arg, err)
		}
		out[i] = buf.String()
	}
	return out, nil
}

// parseLines splits newline-delimited output into a list of non-empty trimmed strings.
func parseLines(data []byte) []string {
	raw := strings.Split(string(data), "\n")
	names := make([]string, 0, len(raw))
	for _, line := range raw {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			names = append(names, trimmed)
		}
	}
	return names
}

// parseJSONList navigates to a JSON path, then extracts the name field from each item.
func parseJSONList(data []byte, path, nameField string) ([]string, error) {
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("failed to parse JSON list output: %w", err)
	}

	node, err := jsonNavigate(root, path)
	if err != nil {
		return nil, fmt.Errorf("failed to navigate JSON path %q: %w", path, err)
	}

	items, ok := node.([]interface{})
	if !ok {
		return nil, fmt.Errorf("JSON path %q does not point to an array", path)
	}

	if nameField == "" {
		nameField = "name"
	}

	names := make([]string, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			names = append(names, v)
		case map[string]interface{}:
			if name, ok := v[nameField].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

// parseJSONValue navigates to a JSON path and returns its string representation.
func parseJSONValue(data []byte, path string) (string, error) {
	var root interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return "", fmt.Errorf("failed to parse JSON get output: %w", err)
	}

	node, err := jsonNavigate(root, path)
	if err != nil {
		return "", fmt.Errorf("failed to navigate JSON path %q: %w", path, err)
	}

	switch v := node.(type) {
	case string:
		return v, nil
	case map[string]interface{}, []interface{}:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal JSON value: %w", err)
		}
		return string(b), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// jsonNavigate follows a dot-separated path through nested maps.
// Only map[string]interface{} nodes are supported; attempting to traverse
// into arrays or other types returns an error.
func jsonNavigate(root interface{}, path string) (interface{}, error) {
	if path == "" {
		return root, nil
	}
	current := root
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key %q not found", part)
			}
			current = val
		default:
			return nil, fmt.Errorf("cannot navigate key %q through %T", part, current)
		}
	}
	return current, nil
}
