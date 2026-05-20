# Tool Backend

The **tool backend** lets you integrate any CLI-based secrets manager with
vaults-syncer without writing custom Go code. A vault of `type: tool` delegates
all operations (list, get, set, delete) to an external program—such as the AWS
CLI, the HashiCorp Vault CLI, or any shell script you provide.

Each tool has its **own configuration file** that you write once and then
reference by path from the main `config.yaml`. This keeps the main config small
and makes tool configs shareable and version-controlled independently.

---

## Quick start

### 1. Write a tool config file

Create a YAML file (e.g. `tools/aws-secretsmanager.yaml`) that describes the
CLI commands for each operation. Ready-made examples are in
[`examples/tools/`](../../examples/tools/).

```yaml
# tools/aws-secretsmanager.yaml
env:
  AWS_DEFAULT_REGION: "${AWS_DEFAULT_REGION:-us-east-1}"

operations:
  list:
    command: aws
    args: [secretsmanager, list-secrets, --output, json]
    output:
      format: json
      path: SecretList
      name_field: Name

  get:
    command: aws
    args: [secretsmanager, get-secret-value, --secret-id, "{{.Name}}", --output, json]
    output:
      format: json
      path: SecretString

  set:
    command: aws
    args: [secretsmanager, put-secret-value, --secret-id, "{{.Name}}", --secret-string, "{{.Value}}"]

  delete:
    command: aws
    args: [secretsmanager, delete-secret, --secret-id, "{{.Name}}", --force-delete-without-recovery]

  test:
    command: aws
    args: [sts, get-caller-identity]
```

### 2. Reference it in `config.yaml`

```yaml
vaults:
  - id: aws_prod
    name: AWS Secrets Manager (prod)
    type: tool
    tool_config: ./tools/aws-secretsmanager.yaml
```

The `tool_config` path is resolved **relative to the directory of the main
config file**. Absolute paths are also accepted.

---

## Tool config file reference

### Top-level fields

| Field | Type | Description |
|---|---|---|
| `env` | `map[string]string` | Environment variables injected into every command. Supports `${VAR}` expansion at config load time. |
| `env_passthrough` | `[]string` | Names of environment variables to forward from the daemon's runtime environment. Values are read at command execution time, so rotated credentials are picked up without restarting. |
| `operations` | `map[string]ToolOperationConfig` | Command definitions keyed by operation name (`list`, `get`, `set`, `delete`, `test`). |

### Operation fields (`operations.<name>`)

| Field | Type | Required | Description |
|---|---|---|---|
| `command` | `string` | ✅ | Executable to run (e.g. `aws`, `vault`, `bash`). |
| `args` | `[]string` | | Arguments passed to the command. Supports Go template variables (see below). |
| `output` | `ToolOutputConfig` | | Describes how to parse stdout. |
| `success_exit_codes` | `[]int` | | Exit codes considered successful. Defaults to `[0]`. |

### Output fields (`operations.<name>.output`)

| Field | Type | Description |
|---|---|---|
| `format` | `string` | `json` (default), `text`, or `lines`. |
| `path` | `string` | Dot-notation path into JSON output (e.g. `SecretList` or `data.keys`). Empty string = root. |
| `name_field` | `string` | For `list`: JSON field within each array item that holds the secret name. Defaults to `name`. Items that are plain strings are used as-is. |

---

## Template variables in args

The `args` array supports **Go template** syntax. The following variables are
available depending on the operation:

| Variable | Available in | Description |
|---|---|---|
| `{{.Name}}` | `get`, `set`, `delete` | The secret name being operated on. |
| `{{.Value}}` | `set` | The secret value to write. |

Example:

```yaml
get:
  command: vault
  args: [kv, get, -field=value, "secret/{{.Name}}"]
```

---

## Required operations

| Operation | Required | Description |
|---|---|---|
| `list` | ✅ | Returns all secret names. |
| `get` | ✅ | Returns the value of a single secret. |
| `set` | Optional | Creates or updates a secret. |
| `delete` | Optional | Removes a secret. |
| `test` | Optional | Used by `TestConnection`. Falls back to `list` if absent. |

---

## Output formats

### `json`

The command output is parsed as JSON. Use `output.path` to navigate to the
relevant node (dot notation). For `list`, the node must be a JSON array; each
element may be a plain string or an object with a `name_field`.

```yaml
output:
  format: json
  path: SecretList     # navigate to {"SecretList": [...]}
  name_field: Name     # extract item["Name"]
```

### `text` (get only)

The raw stdout is trimmed and used as the secret value. Useful for tools like
`vault kv get -field=value`.

### `lines`

Each non-empty line of stdout becomes a secret name (for `list`) or the entire
output is the value (for `get`).

---

## Environment variable injection

Tool configs support two ways to pass environment variables to commands:

### `env` — static values (expanded at load time)

```yaml
env:
  VAULT_ADDR: "${VAULT_ADDR:-http://127.0.0.1:8200}"
```

Values in the `env` block are expanded once when the config is loaded. Use
this for variables whose values are known at daemon startup.

### `env_passthrough` — runtime forwarding

```yaml
env_passthrough:
  - AWS_ACCESS_KEY_ID
  - AWS_SECRET_ACCESS_KEY
  - AWS_SESSION_TOKEN
```

`env_passthrough` reads the named variables from the daemon's environment
**at the moment each command runs**. This means rotated credentials (e.g. an
IAM role's short-lived session tokens refreshed by an agent) are automatically
picked up without restarting the daemon.

If a listed variable is not set in the daemon's environment, it is silently
omitted and does not affect the command.

All variables already present in the daemon's environment are also inherited
by child processes automatically; `env` and `env_passthrough` let you
override or explicitly forward specific variables.

---

## Examples

Pre-built tool configs are provided in [`examples/tools/`](../../examples/tools/):

| File | Description |
|---|---|
| `aws-secretsmanager.yaml` | AWS Secrets Manager via the AWS CLI |
| `hashicorp-vault.yaml` | HashiCorp Vault KV v2 via the `vault` CLI |

---

## Limitations

- The tool backend executes commands synchronously. Long-running CLIs will
  block a sync worker for the duration of the command (up to the vault
  `timeout`, default 30 s).
- stdin is not connected; tools that require interactive input are not
  supported.
- The tool backend does not support bidirectional sync unless both `get` and
  `set` operations are defined.
