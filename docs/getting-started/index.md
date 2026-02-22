# Getting Started

Welcome to Secrets Vault Sync! This guide will help you get up and running in just a few minutes.

## What is Secrets Vault Sync?

Secrets Vault Sync is a daemon that synchronizes secrets between multiple vault backends:

- **Source**: Where secrets are read from (your primary vault)
- **Target(s)**: Where secrets are written to (backup, other environments, different providers)

It supports:
- ✅ Vaultwarden
- ✅ HashiCorp Vault
- ✅ Azure Key Vault
- ✅ AWS Secrets Manager
- ✅ Any HTTP-based secret store

## Prerequisites

- Docker or Go 1.22+ (for source builds)
- A source vault with configuration access
- At least one target vault to sync to
- Basic understanding of YAML configuration

## Next Steps

- [⚡ Quick Start (5 min)](quickstart.md) - Get running immediately
- [📦 Installation](installation.md) - Detailed setup options
