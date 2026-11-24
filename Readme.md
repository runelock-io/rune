<p align="center">
  <img src="./assets/logo.png" alt="Rune Logo" width="200"/>
</p>

<h1 align="center">Rune</h1>

<p align="center">
  <strong>A minimal, secure, self-hosted secrets manager.</strong><br/>
  Lightweight. Encrypted. Local-first.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-v1.0.1--RC-blue" alt="version"/>
  <img src="https://img.shields.io/badge/license-Apache%202.0-green" alt="license"/>
  <img src="https://img.shields.io/badge/go-%3E%3D1.21-00ADD8?logo=go" alt="go version"/>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="platform"/>
</p>

---

## Table of Contents

- [Overview](#overview)
- [What's New in v1.0.1-RC](#whats-new-in-v101-rc)
- [Features](#features)
- [Architecture](#architecture)
- [Installation](#installation)
- [Updating Rune](#updating-rune)
- [Getting Started](#getting-started)
- [Multi-Developer Auth](#multi-developer-auth)
- [Kubernetes Operator](#kubernetes-operator)
- [Deploying on AWS EC2](#deploying-on-aws-ec2)
- [CLI Reference](#cli-reference)
- [API Reference](#api-reference)
- [Data Storage](#data-storage)
- [Security Model](#security-model)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [Philosophy](#philosophy)
- [License](#license)

---

## What's New in v1.0.1-RC

### Multi-Developer Auth
Rune can now be shared across a team without exposing the master passphrase. Admins create username + password accounts for developers via the new **Users** view. Each user gets their own session token scoped to specific namespaces and permissions. Developers log in with familiar credentials instead of pasting a 64-character hex token.

### Auto-Unseal
Set `RUNE_UNSEAL_PASSPHRASE` in your deployment environment and the vault unseals itself automatically on every restart. Developers never see the passphrase screen.

### Kubernetes Operator
Rune now ships a Kubernetes operator (`rune-operator`) that syncs secrets from Rune namespaces directly into Kubernetes Secrets. Create a mapping in the UI or via API — the operator watches for changes and keeps your cluster in sync.

### TTL-Based Auto-Rotation
Secrets can now have a TTL (`30s`, `1h`, `7d`, `90d`, etc.). When a secret expires, the vault's background scheduler rotates it automatically and logs the event in the audit log. Set TTLs from the CLI with `rune put --ttl` or `rune set-ttl`, or from the UI.

### `rune update`
The CLI can now update itself in-place. Run `rune update` and it will check for the latest release on GitHub, download the right binary for your platform, and replace itself atomically.

---

## Overview

Rune is a lightweight, self-hosted secrets manager built for developers who want full control over their secrets without the overhead of complex infrastructure.

It provides AES-256-GCM encrypted storage, a secure seal/unseal lifecycle, namespace-based isolation, secret versioning, and token-based authentication — all in a single binary with zero external dependencies. Access your vault through the built-in web UI or the `rune` CLI.

> **Rune is not a replacement for enterprise secrets management.** It's a focused tool for developers who want a simple, auditable, and self-contained solution.

---

## Features

| Feature | Description |
|---|---|
| **AES-256-GCM Encryption** | Secrets are encrypted at rest using industry-standard authenticated encryption |
| **Argon2id Key Derivation** | Passphrase-based key derivation — resistant to brute-force and GPU attacks |
| **Seal / Unseal Lifecycle** | Vault starts sealed on every launch; no secrets accessible until explicitly unlocked |
| **Token Authentication** | All protected endpoints require a valid token; tokens are SHA-256 hashed before storage |
| **Permission-based Tokens** | Tokens carry per-namespace permissions: `read`, `write`, `delete`, `rotate` |
| **Namespaces** | Isolate secrets by team, service, or environment with fine-grained token access |
| **Secret Versioning** | Full history and point-in-time retrieval for every secret |
| **Secret Rotation** | Rotate secret values on demand; previous versions are preserved |
| **System Tokens** | Read-only tokens with dual-encrypted sealed access for CI/CD and SDK use |
| **Web UI** | Built-in React dashboard for managing secrets, tokens, namespaces, and vault stats |
| **JSON / .env Import** | Bulk-import secrets from a `.json` or `.env` file directly into any namespace |
| **CLI Interface** | Full-featured command-line tool for all vault operations |
| **Zero External Dependencies** | No cloud services, no agents, no sidecars — just a single binary |
| **TTL-Based Auto-Rotation** | Set a TTL on any secret (`30s` min); the scheduler rotates it automatically when it expires |
| **Multi-Developer Auth** | Username + password accounts backed by scoped session tokens — no passphrase sharing |
| **Auto-Unseal** | Set `RUNE_UNSEAL_PASSPHRASE` to unseal the vault automatically on startup |
| **Audit Log** | Tamper-resistant log of every read, write, rotate, login, and admin action |
| **Kubernetes Operator** | Sync Rune secrets into Kubernetes Secrets automatically via the `rune-operator` |
| **BoltDB Storage** | Embedded key-value store — the same underlying engine powering etcd |

---

## Architecture

Rune ships two binaries that work together:

```
┌──────────────────┐              ┌───────────────────────────┐
│   rune CLI       │   HTTP API   │       rune-server         │
│   (client)       │ ───────────► │       :2710               │
└──────────────────┘              │                           │
                                  │  ┌─────────────────────┐  │
┌──────────────────┐              │  │   AES-256-GCM Vault │  │
│   Web Browser    │   HTTP API   │  │   (BoltDB)          │  │
│   /ui/           │ ───────────► │  └─────────────────────┘  │
└──────────────────┘              └───────────────────────────┘
```

- **`rune-server`** — the vault daemon. Manages encryption, exposes the HTTP API, and serves the embedded web UI at `/ui/`
- **`rune`** — the CLI client that communicates with the server over HTTP
- **Web UI** — a React + Tailwind dashboard embedded in the server binary, accessible in any browser

---

## Installation

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runelock-io/rune/main/scripts/install.sh | bash
```

### Verify the installation

```bash
rune --version
rune-server --version
```

> Binary releases for other platforms are available on the [Releases](https://github.com/runelock-io/rune/releases) page.

---

## Updating Rune

### CLI — one command

If you have `rune` v1.0.1-RC or later installed:

```bash
rune update
```

It checks GitHub for the latest release, downloads the right binary for your platform, and replaces itself in-place.

```
Current version: v1.0.1-RC
Checking for updates... done
New version available: v1.0.2
Downloading rune_darwin_arm64... done
✔ Updated to v1.0.2 — restart your shell if needed
```

### CLI — manual (older installs)

If you're on a version before `rune update` was added, download the new binary manually:

```bash
# macOS (Apple Silicon)
curl -L https://github.com/runelock-io/rune/releases/latest/download/rune_darwin_arm64 -o rune
chmod +x rune && sudo mv rune /usr/local/bin/rune

# macOS (Intel)
curl -L https://github.com/runelock-io/rune/releases/latest/download/rune_darwin_amd64 -o rune
chmod +x rune && sudo mv rune /usr/local/bin/rune

# Linux (amd64)
curl -L https://github.com/runelock-io/rune/releases/latest/download/rune_linux_amd64 -o rune
chmod +x rune && sudo mv rune /usr/local/bin/rune

# Linux (arm64)
curl -L https://github.com/runelock-io/rune/releases/latest/download/rune_linux_arm64 -o rune
chmod +x rune && sudo mv rune /usr/local/bin/rune
```

### Server — manual update

```bash
# Stop the server
sudo systemctl stop rune

# Download the new server binary (example: Linux amd64)
curl -L https://github.com/runelock-io/rune/releases/latest/download/rune-server_linux_amd64 -o rune-server
chmod +x rune-server && sudo mv rune-server /usr/local/bin/rune-server

# Restart
sudo systemctl start rune
```

> The database (`~/.rune/rune.db`) is never touched during an update. All secrets, tokens, namespaces, and audit events are preserved.

---

## Getting Started

### Step 1 — Start the server

```bash
rune-server
```

On first launch, Rune prompts for a passphrase and generates a root token:

```
  Set a passphrase: ••••••••••••••

  ────────────────────────────────────────────────────────────────────────────
  root token
  ────────────────────────────────────────────────────────────────────────────

  rnk_92776b83307767ff9d7be381ffae323a07131f98b34e1842fb7f3206771c7b51

  ────────────────────────────────────────────────────────────────────────────
  shown once  ·  not stored  ·  full vault access
  ────────────────────────────────────────────────────────────────────────────
```

> **Save your root token immediately.** It cannot be recovered after this point.

The web UI is available at **[http://localhost:2710/ui/](http://localhost:2710/ui/)** once the server is running.

---

### Step 2 — Unseal the vault

The vault starts **sealed** on every launch. Unseal it with your passphrase:

```bash
rune unseal
```

```
Enter passphrase: ••••••••••••••
✓ Vault unsealed.
```

---

### Step 3 — Login

Authenticate the CLI with your root token:

```bash
rune login rnk_92776b83307767ff9d7be381ffae323a07131f98b34e1842fb7f3206771c7b51
```

The token is stored locally at `~/.rune/config.json`.

---

### Step 4 — Store a secret

```bash
rune put db/password my-secret-value
```

---

### Step 5 — Retrieve a secret

```bash
rune get db/password
```

```
my-secret-value
```

---

### Step 6 — Seal the vault

When you're done, seal the vault to make all secrets inaccessible:

```bash
rune seal
```

```
✓ Vault sealed.
```

---

## Multi-Developer Auth

Rune supports shared deployments where multiple developers each have their own account — without anyone seeing the master passphrase.

### How it works

1. **Admin** deploys Rune and sets `RUNE_UNSEAL_PASSPHRASE` in the server environment (systemd, docker-compose, etc.)
2. **Admin** creates user accounts in the **Users** view (root-only), assigning each user a username, password, and namespace permissions
3. **Developer** opens the Rune UI → logs in with username + password
4. The server returns a scoped session token — the developer never touches the raw token

Each user account has its own session token with the exact namespace permissions the admin configured. Revoking a user immediately invalidates their session.

### Auto-unseal

Add `RUNE_UNSEAL_PASSPHRASE` to your server environment so the vault unseals itself on every restart:

```bash
# systemd unit
[Service]
ExecStart=/usr/local/bin/rune-server
Environment=RUNE_UNSEAL_PASSPHRASE=your-passphrase
```

```bash
# Docker
docker run -e RUNE_UNSEAL_PASSPHRASE=your-passphrase runelock/rune-server
```

If the env var is set but the passphrase is wrong, the server logs a warning and starts sealed — it never silently fails.

### Creating a user (API)

```bash
curl -X POST http://localhost:2710/user/create \
  -H "Authorization: Bearer <root-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "password": "correct-horse-battery",
    "namespaces": [
      {"namespace": "production", "permissions": ["read"]},
      {"namespace": "staging",    "permissions": ["read", "write", "rotate"]}
    ]
  }'
```

### Developer login (API)

```bash
curl -X POST http://localhost:2710/user/login \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "password": "correct-horse-battery"}'

# → {"token": "rnk_..."}
```

The returned token flows through the existing auth middleware — the developer can then use it with the CLI or any SDK.

---

## Kubernetes Operator

The `rune-operator` syncs secrets from a Rune namespace into native Kubernetes Secrets automatically. Configure a mapping once — the operator watches for changes and keeps your cluster in sync.

### How it works

1. Run `rune-operator` in your cluster (it connects to your Rune server over HTTP)
2. Create a **mapping** in the Rune UI (**Kubernetes** view) or via API
3. The operator reads the specified keys from the Rune namespace and writes them into a Kubernetes Secret in the target namespace
4. On each sync cycle (configurable interval), it re-reads from Rune and updates the K8s Secret if anything changed

### Creating a mapping (API)

```bash
curl -X POST http://localhost:2710/k8s/mapping/create \
  -H "Authorization: Bearer <root-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name":              "prod-db-creds",
    "rune_namespace":    "production",
    "keys":              ["db/password", "db/host", "api/key"],
    "target_secret":     "prod-secrets",
    "target_k8s_ns":     "default",
    "secret_type":       "Opaque",
    "refresh_interval":  "5m"
  }'
```

| Field | Description |
|---|---|
| `name` | Human-readable name for the mapping |
| `rune_namespace` | The Rune namespace to read secrets from |
| `keys` | List of secret keys to sync (e.g. `db/password`) |
| `target_secret` | Name of the Kubernetes Secret to create/update |
| `target_k8s_ns` | Kubernetes namespace where the Secret lives |
| `secret_type` | Kubernetes Secret type — `Opaque`, `kubernetes.io/tls`, etc. |
| `refresh_interval` | How often the operator re-syncs (e.g. `1m`, `5m`, `1h`) |

### Checking sync status

```bash
curl http://localhost:2710/k8s/mapping/status \
  -H "Authorization: Bearer <root-token>"

# → {"mapping_id": "...", "last_sync": "...", "status": "synced", "secret_count": 3}
```

Sync status is also visible in the **Kubernetes** view in the web UI, showing `synced`, `pending`, or `error` per mapping.

### Running the operator

```bash
# Point it at your Rune server
RUNE_SERVER=http://localhost:2710 \
RUNE_TOKEN=<root-or-system-token> \
./rune-operator
```

For production, run it as a Kubernetes Deployment with the token stored in a Kubernetes Secret.

---

## Deploying on AWS EC2

This section covers running `rune-server` on an EC2 instance so you can access your vault from a browser or the CLI from any machine.

### Step 1 — Launch an EC2 instance

Any instance type works. Amazon Linux 2023 or Ubuntu 22.04 are both fine. During or after launch, configure the **Security Group** to allow inbound traffic on port `2710`:

| Type | Protocol | Port | Source |
|------|----------|------|--------|
| Custom TCP | TCP | 2710 | Your IP (`x.x.x.x/32`) |
| SSH | TCP | 22 | Your IP (`x.x.x.x/32`) |

> Restrict port 2710 to your own IP — do not open it to `0.0.0.0/0` unless you have a firewall or reverse proxy in front. Rune currently serves plain HTTP.

---

### Step 2 — Install Rune on the instance

SSH into your instance and run the installer:

```bash
ssh -i your-key.pem ec2-user@<ec2-public-ip>

curl -fsSL https://raw.githubusercontent.com/runelock-io/rune/main/scripts/install.sh | bash
```

Verify the install:

```bash
rune-server --version
```

---

### Step 3 — Run the server

Start `rune-server` for the first time. It will prompt for a passphrase and print your root token:

```bash
rune-server
```

```
Set a passphrase: ••••••••••••••

  ────────────────────────────────────────────────────
  root token
  ────────────────────────────────────────────────────

  rnk_92776b83307767ff9d7be381ffae323a07131f98b34e1842fb7f3206771c7b51

  ────────────────────────────────────────────────────
  shown once  ·  not stored  ·  full vault access
  ────────────────────────────────────────────────────
```

> **Save your root token and passphrase immediately.** Neither can be recovered.

The server binds to `0.0.0.0:2710` — it is now reachable at `http://<ec2-public-ip>:2710`.

---

### Step 4 — Run as a background service (systemd)

To keep the server running after you disconnect, create a systemd unit:

```bash
sudo tee /etc/systemd/system/rune.service > /dev/null <<EOF
[Unit]
Description=Rune Secrets Vault
After=network.target

[Service]
ExecStart=/usr/local/bin/rune-server
Restart=on-failure
RestartSec=5
User=ec2-user
WorkingDirectory=/home/ec2-user
StandardOutput=journal
StandardError=journal
# Optional: auto-unseal on startup so developers never see the passphrase screen
# Environment=RUNE_UNSEAL_PASSPHRASE=your-passphrase

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable rune
sudo systemctl start rune
```

Check it is running:

```bash
sudo systemctl status rune
```

> On first start the service needs terminal input for the passphrase prompt. Run `rune-server` manually once to complete the initial setup, then start the service. On subsequent starts the passphrase is entered via `rune unseal` after the service is up.

---

### Step 5 — Unseal after a restart

The vault starts **sealed** every time the process launches. After the service starts (or the instance reboots), unseal it remotely from your local machine:

```bash
rune --server http://<ec2-public-ip>:2710 unseal
```

Or set the env var so you don't have to repeat the flag:

```bash
export RUNE_SERVER=http://<ec2-public-ip>:2710
rune unseal
```

---

### Step 6 — Connect the CLI from your local machine

Log in once to save the server URL and token together:

```bash
rune login --server http://<ec2-public-ip>:2710 rnk_92776b83...
```

All subsequent CLI commands now target the EC2 instance automatically — no flags needed:

```bash
rune put DATABASE_URL postgres://...
rune list
rune get DATABASE_URL
```

---

### Step 7 — Open the Web UI

Navigate to:

```
http://<ec2-public-ip>:2710/ui/
```

Enter your root token to log in.

---

### Production hardening checklist

| | Recommendation |
|---|---|
| **TLS** | Put Rune behind Nginx or Caddy with a TLS certificate (Let's Encrypt). Rune itself serves plain HTTP — TLS termination at the reverse proxy is the standard pattern. |
| **Firewall** | Restrict port 2710 in the Security Group to known IPs. If using a reverse proxy on port 443, close 2710 to the public entirely. |
| **Persistence** | The database lives at `~/.rune/rune.db`. Back it up to S3 with a cron job or AWS Backup. |
| **IAM** | Do not run `rune-server` as root. Use a dedicated system user with no other privileges. |
| **Unseal automation** | Set `RUNE_UNSEAL_PASSPHRASE` in the systemd unit. Store the passphrase value in AWS SSM Parameter Store and inject it at deploy time — never hardcode it in the unit file on disk. |

---

## CLI Reference

```
USAGE:
  rune <command> [arguments]

VAULT
  put <key> <value>               Store a secret (use -n for namespace, -t for TTL)
  get <key>                       Retrieve a secret
  list                            List all secrets
  delete <key>                    Delete a secret
  rotate <key>                    Rotate a secret and preserve the old version
  history <key>                   Show all versions of a secret
  set-ttl <key> [ttl]             Set auto-rotation TTL on a secret (e.g. 7d, 24h, 30s)

ACCESS
  unseal                          Unseal the vault using your passphrase
  seal                            Seal the vault, making all secrets inaccessible
  status                          Display the current seal status of the vault

ORG
  namespace create <name>         Create a new namespace
  namespace list                  List all namespaces
  namespace delete <name>         Delete a namespace
  token create <name>             Create a token with per-namespace permission grants
  token list                      List all tokens
  token revoke <id>               Revoke a token by ID
  token create-system <name>      Create a system token (read-only, sealed-access capable)
  login <token>                   Authenticate with the vault and store token locally
  update                          Update the CLI to the latest release

OPTIONS:
  -n, --namespace <name>          Target namespace for secret operations
  -s, --server <url>              Rune server URL — used with login to point at a remote server
  -t, --ttl <duration>            Auto-rotation TTL — used with put (e.g. 7d, 24h, 30s)
  --help                          Show help for any command
  --version                       Print the current version

ENVIRONMENT:
  RUNE_SERVER                     Override the server URL for all commands (e.g. http://1.2.3.4:2710)
  RUNE_UNSEAL_PASSPHRASE          Auto-unseal the vault on server startup (server-side env var)
```

### Examples

```bash
# Start the server
rune-server

# Unseal the vault
rune unseal

# Authenticate
rune login rnk_92776b83307767ff9d7be381ffae323a07131f98b34e1842fb7f3206771c7b51

# Secrets — default namespace
rune put db/password my-secret-password
rune put api/key sk_live_abc123
rune get db/password
rune list
rune delete db/password

# Secrets — named namespace
rune namespace create springboot
rune put db/password supersecret -n springboot
rune get db/password -n springboot
rune list -n springboot
rune delete db/password -n springboot
rune namespace delete springboot

# Versioning and rotation
rune history db/password
rune rotate db/password

# Tokens
rune token create backend-service --namespace springboot
rune token create-system ci-reader --namespace springboot
rune token list
rune token revoke <token-id>

# Seal when done
rune seal
```

---

## API Reference

All endpoints listen on `http://localhost:2710`. Protected endpoints require a Bearer token.

```
Authorization: Bearer <your-token>
```

### Public

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/status` | Vault seal status |
| `POST` | `/unseal` | Unseal the vault |
| `GET` | `/health` | Health check |
| `POST` | `/user/login` | Log in with username + password, returns session token |

### Secrets

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/secret/put` | Store or update a secret |
| `GET` | `/secret/get` | Retrieve the latest value of a secret |
| `POST` | `/secret/delete` | Delete a secret |
| `GET` | `/secret/list` | List secrets in a namespace |
| `GET` | `/secret/meta` | List secrets with metadata (version, TTL, expiry) |
| `GET` | `/secret/all` | Retrieve all secrets (key + value) in a namespace |
| `POST` | `/secret/rotate` | Rotate a secret and preserve the old version |
| `GET` | `/secret/history` | List all versions of a secret |
| `GET` | `/secret/version` | Retrieve a specific version of a secret |
| `POST` | `/secret/ttl` | Set or clear the auto-rotation TTL on a secret |

### Namespaces

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/namespace/create` | Create a namespace |
| `GET` | `/namespace/list` | List all namespaces |
| `POST` | `/namespace/delete` | Delete a namespace |

### Tokens

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/token/me` | Get info about the current token |
| `POST` | `/token/create` | Create a token with per-namespace permissions |
| `POST` | `/token/create-system` | Create a read-only system token |
| `POST` | `/token/revoke` | Revoke a token |
| `GET` | `/token/list` | List all tokens (root only) |

### Users

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/user/create` | Create a user account with namespace access (root only) |
| `GET` | `/user/list` | List all user accounts (root only) |
| `POST` | `/user/delete` | Delete a user account and revoke their token (root only) |

### Kubernetes

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/k8s/mapping/create` | Create a Rune → Kubernetes Secret mapping |
| `GET` | `/k8s/mappings` | List all mappings with sync status |
| `POST` | `/k8s/mapping/delete` | Delete a mapping |
| `POST` | `/k8s/mapping/status` | Update the sync status of a mapping (operator use) |

### Audit

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/audit/events` | List audit events (root only, `?limit=N`) |

### Vault

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/seal` | Seal the vault |
| `GET` | `/stats` | Vault statistics (root only) |

### Example Requests

```bash
# Check vault status
curl http://localhost:2710/status

# Unseal
curl -X POST http://localhost:2710/unseal \
  -H "Content-Type: application/json" \
  -d '{"passphrase": "your-passphrase"}'

# Store a secret
curl -X POST http://localhost:2710/secret/put \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"namespace": "default", "key": "db/password", "value": "my-secret"}'

# Retrieve a secret
curl "http://localhost:2710/secret/get?namespace=default&key=db/password" \
  -H "Authorization: Bearer <token>"

# Get all secrets in a namespace
curl "http://localhost:2710/secret/all?namespace=default" \
  -H "Authorization: Bearer <token>"

# Rotate a secret
curl -X POST "http://localhost:2710/secret/rotate?namespace=default&key=db/password" \
  -H "Authorization: Bearer <token>"

# Create a token with specific permissions
curl -X POST http://localhost:2710/token/create \
  -H "Authorization: Bearer <root-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "backend-service",
    "namespaces": [
      {"namespace": "production", "permissions": ["read"]},
      {"namespace": "staging", "permissions": ["read", "write", "delete", "rotate"]}
    ]
  }'
```

---

## Data Storage

Rune stores all data locally. No data ever leaves your machine unless you choose to run `rune-server` on a remote host.

```
~/.rune/
├── rune.db        # AES-256-GCM encrypted secrets (BoltDB)
└── config.json    # CLI authentication token
```

Secret keys are stored in BoltDB using the format `{namespace}/{key}` for the latest value and `{namespace}/{key}@v{N}` for versioned history. All values are encrypted before being written. The encryption key is derived at unseal time and held only in memory — it is never written to disk.

---

## Security Model

Rune's security design prioritizes simplicity and auditability over complexity.

**Encryption**
- Secrets are encrypted using **AES-256-GCM**, providing both confidentiality and integrity.
- Each secret is encrypted with a unique nonce to prevent ciphertext reuse.

**Key Derivation**
- The encryption key is derived from your passphrase using **Argon2id**, a memory-hard KDF that resists brute-force and GPU-based attacks.
- The derived key is held in memory only while the vault is unsealed. It is never written to disk.

**Seal / Unseal**
- The vault starts **sealed** on every process launch.
- In a sealed state, no secret can be read or written — the in-memory key is zeroed out.
- Sealing is instantaneous and does not require a restart.

**Authentication**
- Tokens are **SHA-256 hashed before storage**. The server never stores raw tokens.
- All read and write operations on secrets require a valid, authenticated token.
- Tokens carry explicit per-namespace permissions (`read`, `write`, `delete`, `rotate`). Only the root token (`namespace: *`) has unrestricted access.
- System tokens (`stk_` prefix) use dual-encryption to support sealed-state reads in CI/CD pipelines.

**Threat model:** Rune protects secrets at rest from an attacker with access to the filesystem. It is not designed to protect against a compromised process, root-level access, or memory forensics against a live, unsealed vault.

---

## Roadmap

- [x] Token management (create, revoke, list)
- [x] Namespaces for multi-tenant secret isolation
- [x] Secret versioning and history
- [x] Secret rotation
- [x] Web UI dashboard
- [x] JSON / .env bulk import
- [x] Permission-based token access control
- [x] System tokens with sealed-state access
- [x] Audit logging
- [x] Role-based access control (RBAC)
- [x] TTL-based auto-rotation
- [x] Kubernetes Operator (sync to K8s Secrets)
- [x] Multi-developer auth (username + password accounts)
- [x] Auto-unseal via environment variable
- [x] CLI self-update (`rune update`)
- [ ] TLS support built-in (currently via reverse proxy)
- [ ] HashiCorp Vault import
- [ ] Brute-force protection on login/unseal endpoints
- [ ] CLI self-update for `rune-server`

---

## Contributing

Contributions are welcome and appreciated.

1. **Fork** the repository
2. **Create** a feature branch: `git checkout -b feat/my-feature`
3. **Commit** your changes: `git commit -m "feat: add my feature"`
4. **Push** to your branch: `git push origin feat/my-feature`
5. **Open** a pull request

Please open an issue first for significant changes or new features so we can discuss the approach before you invest time in the implementation.

### Reporting Issues

If you discover a security vulnerability, **do not open a public issue**. Please email [security@runelock.io](mailto:security@runelock.io) instead.  
For bugs and feature requests, use the [GitHub issue tracker](https://github.com/runelock-io/rune/issues).

---

## Philosophy

> *Secure systems do not need to be complex.*

Rune exists because most secrets managers either require cloud infrastructure, heavy runtimes, or significant operational overhead. Rune is a single binary. Its source is small enough to read in an afternoon.

It is built with three principles in mind:

**Minimalism** — every feature must justify its existence. Complexity is a liability.  
**Control** — your secrets run on your infrastructure. Nothing phones home.  
**Clarity** — the security model is simple enough to be understood and audited by a single developer.

Rune is inspired by [HashiCorp Vault](https://www.vaultproject.io/) and [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/), but intentionally scoped for local and small-team use cases where those tools are too heavy.

---

## License

Rune is released under the [Apache License 2.0](./LICENSE).

---

<p align="center">
  Built with focus on simplicity and real-world usability.<br/>
  If Rune is useful to you, a ★ on <a href="https://github.com/runelock-io/rune">GitHub</a> goes a long way.
</p>
