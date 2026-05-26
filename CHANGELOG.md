# Changelog

## [4.2.0](https://github.com/pacorreia/vaults-syncer/compare/v4.1.1...v4.2.0) (2026-05-26)


### Features

* add DB-backed config, auth, security, and new API endpoints ([91db1c7](https://github.com/pacorreia/vaults-syncer/commit/91db1c7d5beaed39641e50b09c79c19bff6ed752))
* add lint/security workflows, Helm chart, Helm OCI publish on release ([91e833d](https://github.com/pacorreia/vaults-syncer/commit/91e833d2cd0e769471b37a16e4e32ee05af48427))
* frontend-based configuration with DB-backed storage, auth, encryption, and CI/CD pipelines ([17e6078](https://github.com/pacorreia/vaults-syncer/commit/17e6078a2528e5909e414c49fc1476f0c76f8ac0))


### Bug Fixes

* address code review comments (error wrapping, goroutine recovery, config race doc) ([91462c9](https://github.com/pacorreia/vaults-syncer/commit/91462c9f0159184c2f778516d9d08b282f169907))
* address code review feedback for DB placeholders, error handling, and reload logic ([9cf988b](https://github.com/pacorreia/vaults-syncer/commit/9cf988bea49ab0c1095c666c923190657fa9278a))
* eliminate data races in TestCompleteSetup and TestCreateAndGetVault ([143246e](https://github.com/pacorreia/vaults-syncer/commit/143246e7629050f1af89ae2428af98006b192be7))
* golangci-lint v2.12.2 config schema and integration test vault endpoints ([e90a235](https://github.com/pacorreia/vaults-syncer/commit/e90a23599c2edfcdbac7e78a5f343230adbaac7e))
* **lint:** correct multi-line comment period placement in config/types.go ([4a29ed1](https://github.com/pacorreia/vaults-syncer/commit/4a29ed148285b069a0e6cf80e9d2af0527cf2f16))
* **lint:** move goimports to formatters, fix all godot/misspell/staticcheck issues ([e96ce0e](https://github.com/pacorreia/vaults-syncer/commit/e96ce0e47d62b021e4c38a51887e79351aa79a0a))
* resolve all CI failures - lint config, integration tests, Go vulnerabilities ([becbae9](https://github.com/pacorreia/vaults-syncer/commit/becbae9eecafd26c4eaeab92545802baf8645476))
* resolve CI failures - golangci-lint v2 config and security workflow auth error ([65358ff](https://github.com/pacorreia/vaults-syncer/commit/65358ff5e127045ce2aa19d530c3df847f5caea7))
* route prefix, runner hot-swap, and golangci-lint v2.12.2 schema ([e017ca9](https://github.com/pacorreia/vaults-syncer/commit/e017ca9f22a1c5b3e68fd081017fcb2313d652dc))

## [4.1.1](https://github.com/pacorreia/vaults-syncer/compare/v4.1.0...v4.1.1) (2026-05-22)


### Bug Fixes

* **workflows:** remove outdated v4 download-artifact comments ([48e8805](https://github.com/pacorreia/vaults-syncer/commit/48e88056b8b79b71f68a105e82e814b65fc1c23a))

## [4.1.0](https://github.com/pacorreia/vaults-syncer/compare/v4.0.0...v4.1.0) (2026-05-22)


### Features

* add env_passthrough to tool backend for runtime env var forwarding ([c547e1f](https://github.com/pacorreia/vaults-syncer/commit/c547e1f5c26fdb23bb53a953305e5f06a96a2a9f))
* tool backend + SRP config refactor + unit tests + docs ([e93a54b](https://github.com/pacorreia/vaults-syncer/commit/e93a54b6848c69b5293f7b0f76786a152898a7c0))
* tool backend with per-tool config files + SRP refactor of config package ([7ae6737](https://github.com/pacorreia/vaults-syncer/commit/7ae673754e00c2019111af64727a95d46666d507))


### Bug Fixes

* remove $schema from release-please manifest ([933bce4](https://github.com/pacorreia/vaults-syncer/commit/933bce468cfdfb5a60b21cd38dc8b54e29c3ca09))
* remove $schema from release-please manifest to fix parsing error ([dd3c50c](https://github.com/pacorreia/vaults-syncer/commit/dd3c50cea3de25fb23a011acc600c7ac7dd1f369))
* remove duplicate github-actions entry from dependabot config ([6dbffe5](https://github.com/pacorreia/vaults-syncer/commit/6dbffe55548d1c0792f79affec67ec9afd26d924))
* remove unused time import, deduplicate cmd env, SyncConfig.Enabled *bool ([81840f5](https://github.com/pacorreia/vaults-syncer/commit/81840f51cc85bd0519056dc1edbfc223fa38f2e3))
* renderArgs missingkey=error, jsonNavigate docstring, sync-daemon binary naming ([0f7d8da](https://github.com/pacorreia/vaults-syncer/commit/0f7d8dabc26c6c694e8466c62c18169dab178758))
