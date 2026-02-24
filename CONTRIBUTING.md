# Contributing to vaults-syncer

Thank you for considering contributing to vaults-syncer! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites

- Go 1.22.1 or later
- Docker and Docker Compose
- Git

### Setting Up Development Environment

```bash
# Clone the repository
git clone https://github.com/pacorreia/vaults-syncer.git
cd vaults-syncer

# Install dependencies
go mod download

# Build the project
go build -o vaults-syncer .

# Run tests
go test ./...

# Run integration tests (requires Docker)
docker-compose -f docker-compose.test.yml up --abort-on-container-exit
```

## How to Contribute

### Reporting Issues

- Check existing issues before creating a new one
- Use the issue template when available
- Include relevant details: Go version, OS, configuration, logs
- For security issues, see [Security Policy](SECURITY.md)

### Submitting Pull Requests

1. **Fork the repository** and create your branch from `main`

2. **Make your changes**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

3. **Follow coding standards**:
   - Run `go fmt` on your code
   - Follow Go best practices
   - Add comments for exported functions/types
   - Write unit tests for new functionality

4. **Test your changes**:
   ```bash
   # Run all tests
   go test -v ./...
   
   # Run with race detector
   go test -race ./...
   
   # Check coverage
   go test -cover ./...
   ```

4. **Commit your changes** with clear, semantic commit messages:
   ```bash
   # Patch bump (default) - fixes, docs, chores
   git commit -m "fix: correct OAuth2 token refresh logic"
   git commit -m "docs: update configuration examples"
   git commit -m "chore: update dependencies"
   
   # Minor bump - new features
   git commit -m "feat: add support for AWS Secrets Manager"
   git commit -m "[minor] implement webhook triggers"
   
   # Major bump - breaking changes
   git commit -m "BREAKING CHANGE: refactor configuration format"
   git commit -m "[major] remove deprecated v1 API"
   ```

5. **Push to your fork** and create a pull request:
   ```bash
   git push origin feature/your-feature-name
   ```

### PR Guidelines

- Use the pull request template
- Include a clear description of changes
- Reference related issues (e.g., "Closes #123")
- Keep PRs focused on a single feature/fix
- Update documentation as needed
- Add tests for new functionality
- Ensure all tests pass
- Use semantic commit messages for proper versioning

## Versioning

This project uses **automated semantic versioning** based on commit messages.

### Version Bump Rules

The version is automatically determined by your commit messages when pushed to `main`:

| Bump Type | Trigger | Example | Result |
|-----------|---------|---------|--------|
| **Patch** | Default (no keyword) | `fix: resolve sync error` | v1.2.3 → v1.2.4 |
| **Minor** | `feat:` or `[minor]` in message | `feat: add new vault type` | v1.2.3 → v1.3.0 |
| **Major** | `BREAKING CHANGE:` or `[major]` | `BREAKING CHANGE: new config format` | v1.2.3 → v2.0.0 |

### Commit Message Examples

```bash
# Patch bumps
✅ fix: resolve sync error
✅ docs: update authentication examples
✅ chore: update dependencies
✅ refactor: simplify vault client code

# Minor bumps
✅ feat: add Google Secret Manager support
✅ [minor] implement webhook triggers
✅ feat: add rate limiting for vault operations

# Major bumps
✅ BREAKING CHANGE: refactor configuration schema
✅ [major] remove deprecated v1 API endpoints
✅ feat!: change authentication flow (BREAKING CHANGE)
```

### Using Conventional Commits

We recommend following the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Common types:**
- `feat:` - New feature (minor bump)
- `fix:` - Bug fix (patch bump)
- `docs:` - Documentation only
- `style:` - Code style changes (formatting, etc.)
- `refactor:` - Code refactoring
- `perf:` - Performance improvements
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks

### What Happens After Merge

1. **Version Bump workflow** (triggers on push to main):
   - Analyzes all commits since last tag
   - Determines version bump type
   - Creates and pushes a git tag (e.g., `v1.3.0`)

2. **Release workflow** (triggered by new tag):
   - Builds binaries for all platforms
   - Creates Docker images (multi-arch)
   - Generates release notes with changelog
   - Publishes GitHub release with assets

## Code Style

- Follow standard Go conventions
- Use `go fmt` for formatting
- Run `go vet` to catch common issues
- Use meaningful variable/function names
- Add comments for exported APIs
- Keep functions focused and testable

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run specific package
go test ./vault

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Integration Tests

```bash
# Full integration test suite
docker-compose -f docker-compose.test.yml up --abort-on-container-exit

# Clean up
docker-compose -f docker-compose.test.yml down -v
```

## Documentation

- Update README.md for user-facing changes
- Update docs/ for detailed documentation
- Add inline comments for complex logic
- Update examples in `config.example.yaml`
- Include usage examples in docstrings

## Areas for Contribution

We welcome contributions in these areas:

### High Priority

- [ ] Additional vault backends (Google Secret Manager, 1Password, etc.)
- [ ] Web UI for configuration and monitoring
- [ ] Webhook-based triggers
- [ ] Enhanced secret transformation capabilities
- [ ] Multi-tenant support

### Medium Priority

- [ ] Grafana dashboards for metrics
- [ ] Helm charts for Kubernetes deployment
- [ ] Secret encryption at rest
- [ ] Advanced conflict resolution strategies
- [ ] Rate limiting for vault operations

### Good First Issues

Look for issues labeled `good-first-issue` or `help-wanted`:
- Documentation improvements
- Test coverage improvements
- Code refactoring
- Bug fixes

## Community

- **Discussions**: Use GitHub Discussions for questions and ideas
- **Issues**: Report bugs and request features via GitHub Issues
- **Pull Requests**: Submit code contributions via pull requests

## License

By contributing to vaults-syncer, you agree that your contributions will be licensed under the MIT License.

## Questions?

Feel free to open a Discussion or reach out via Issues if you have questions about contributing!

Thank you for contributing! 🎉
