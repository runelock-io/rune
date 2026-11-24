# Rune Kubernetes Operator

The Rune Kubernetes Operator syncs secrets from a Rune vault into native Kubernetes `Secret` objects. Rune stays the source of truth — the operator polls it on a configurable interval and creates or updates K8s Secrets automatically.

---

## How it works

```
┌─────────────────────────┐        poll every N seconds        ┌──────────────────────┐
│     Rune Server         │ ◄─── GET /k8s/mappings ──────────  │   rune-operator      │
│  (source of truth)      │                                     │  (in-cluster pod)    │
│                         │ ◄─── GET /secret/all?namespace=X ─  │                      │
│  Namespaces + Secrets   │                                     │  reads mappings,     │
│  Mapping definitions    │ ◄─── POST /k8s/mapping/status ────  │  syncs K8s Secrets,  │
│                         │                                     │  reports status back │
└─────────────────────────┘                                     └──────────┬───────────┘
                                                                           │ kubectl apply
                                                                           ▼
                                                                ┌──────────────────────┐
                                                                │  Kubernetes Secrets  │
                                                                │  (managed-by: rune)  │
                                                                └──────────────────────┘
```

**Key design decisions:**

- Rune never talks to Kubernetes — it has no K8s credentials. The operator pulls, Rune serves.
- The operator is stateless. All mapping config lives in Rune's BoltDB.
- The K8s system token is **wildcard** (`namespace: *`) — it can read all Rune namespaces so a single operator deployment covers the whole vault.
- Wildcard system tokens require the vault to be **unsealed** — unlike per-namespace system tokens, they do not use dual-encryption. If you seal the vault, the operator stops syncing until it is unsealed again.

---

## Prerequisites

| Requirement | Notes |
|---|---|
| Rune server running and unsealed | Port `2710` by default |
| Docker | For building the operator image |
| `kubectl` | Connected to your cluster |
| A Kubernetes cluster | `kind` works for local dev; any cluster works for production |

---

## Step 1 — Create a Kubernetes system token

Open the Rune UI → **Tokens** → **System Token**.

1. Select the **Kubernetes** platform card.
2. Give it a name (e.g. `k8s-operator-prod`).
3. Click **Create Token**.
4. Copy the token from the reveal modal — it is shown **once only**.

> The Kubernetes platform automatically uses a wildcard namespace (`*`), giving the operator read access across all Rune namespaces. No dual-encryption is applied — the vault must remain unsealed for the operator to function.

---

## Step 2 — Build the operator binary and Docker image

Add the operator build targets to your `Makefile` (or run the commands directly):

```makefile
operator-build: $(BIN)
	go build -o $(BIN)/rune-operator ./cmd/operator

operator-image: operator-build
	docker build -f Dockerfile.operator -t rune-operator:dev .
```

Then build:

```bash
make operator-build
make operator-image
```

For a **kind** cluster, load the image directly (no registry needed):

```bash
kind load docker-image rune-operator:dev --name <your-cluster-name>
```

For a **real cluster**, push to your container registry:

```bash
docker tag rune-operator:dev your-registry/rune-operator:v1.0.0-RC
docker push your-registry/rune-operator:v1.0.0-RC
```

Then update the `image:` field in `config/operator/deploy.yaml` accordingly.

---

## Step 3 — Deploy to Kubernetes

### 3a. Create the credentials secret

Replace `<YOUR_SYSTEM_TOKEN>` with the token you copied in Step 1.

```bash
kubectl create namespace rune-system

kubectl create secret generic rune-operator-credentials \
  --namespace rune-system \
  --from-literal=RUNE_SERVER="http://host.docker.internal:2710" \
  --from-literal=RUNE_TOKEN="<YOUR_SYSTEM_TOKEN>"
```

> **macOS / Docker Desktop:** Use `host.docker.internal` to reach the Rune server running on your Mac from inside the cluster.
>
> **Linux / bare-metal:** Use the host's LAN IP or a NodePort service instead of `host.docker.internal`.
>
> **Production:** Replace the URL with the internal DNS name or LoadBalancer address of your Rune server.

### 3b. Apply the manifests

```bash
kubectl apply -f config/operator/deploy.yaml
```

This creates:

| Resource | Name | Namespace |
|---|---|---|
| `Namespace` | `rune-system` | — |
| `ServiceAccount` | `rune-operator` | `rune-system` |
| `ClusterRole` | `rune-operator` | cluster-wide |
| `ClusterRoleBinding` | `rune-operator` | cluster-wide |
| `Deployment` | `rune-operator` | `rune-system` |

The ClusterRole grants the operator `get`, `list`, `create`, `update`, `patch`, and `delete` on `secrets` across all namespaces.

### 3c. Verify the operator is running

```bash
kubectl get pods -n rune-system
# NAME                             READY   STATUS    RESTARTS   AGE
# rune-operator-xxxxxxxxx-xxxxx    1/1     Running   0          30s

kubectl logs -n rune-system -l app=rune-operator --follow
# rune-operator started  server=http://host.docker.internal:2710  interval=30s
# synced mapping "app-secrets" (default/app-secrets) — 4 keys in 12ms
```

---

## Step 4 — Create a mapping

A **mapping** tells the operator which Rune namespace to read from and which Kubernetes Secret to write to.

Open the Rune UI → **Kubernetes** → **New Mapping**.

| Field | Description | Example |
|---|---|---|
| **Name** | Human-readable label for the mapping | `app-secrets` |
| **Rune Namespace** | The Rune namespace to read secrets from | `production` |
| **Keys** | Specific keys to sync (leave empty to sync all) | `DB_URL`, `API_KEY` |
| **Target Secret** | Name of the K8s Secret to create/update | `app-secrets` |
| **Target K8s Namespace** | Kubernetes namespace to place the Secret in | `default` |
| **Secret Type** | K8s Secret type | `Opaque`, `kubernetes.io/tls`, etc. |
| **Refresh Interval** | How often the operator re-syncs this mapping | `30s`, `1m`, `5m` |

Once created, the operator picks it up on its next poll cycle and syncs the secret immediately.

---

## Step 5 — Use the synced secret in your workloads

Rune-managed secrets are standard Kubernetes Secrets with the label `managed-by: rune-operator`. Use them exactly like any other K8s Secret:

```yaml
# As environment variables
envFrom:
  - secretRef:
      name: app-secrets

# As individual keys
env:
  - name: DATABASE_URL
    valueFrom:
      secretKeyRef:
        name: app-secrets
        key: DB_URL

# As a mounted volume
volumes:
  - name: secrets
    secret:
      secretName: app-secrets
```

---

## Configuration reference

### Operator environment variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `RUNE_SERVER` | Yes | — | Base URL of the Rune server, e.g. `http://rune.internal:2710` |
| `RUNE_TOKEN` | Yes | — | Wildcard Kubernetes system token |
| `SYNC_INTERVAL` | No | `30s` | How often the operator polls for mappings and syncs |

### Mapping fields

| Field | Type | Description |
|---|---|---|
| `name` | string | Display name shown in the Rune UI |
| `rune_namespace` | string | Rune namespace to read from |
| `keys` | `[]string` | Keys to sync; empty array = all keys in the namespace |
| `target_secret` | string | K8s Secret name to create or update |
| `target_k8s_ns` | string | Kubernetes namespace for the Secret |
| `secret_type` | string | K8s Secret type (default: `Opaque`) |
| `refresh_interval` | string | Duration string, e.g. `30s`, `2m` (informational — actual polling is driven by `SYNC_INTERVAL`) |

---

## API reference

These endpoints are used internally by the operator and the Rune UI. All require a Bearer token.

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/k8s/mappings` | Root | List all mappings with current sync status |
| `POST` | `/k8s/mapping/create` | Root | Create a new mapping |
| `POST` | `/k8s/mapping/delete` | Root | Delete a mapping by `id` |
| `POST` | `/k8s/mapping/status` | Any token | Operator reports sync result back to Rune |

---

## Sync lifecycle

```
Operator starts
    │
    ▼
GET /k8s/mappings  ──► for each mapping:
    │                      GET /secret/all?namespace=<rune_namespace>
    │                      apply key filter (if keys != [])
    │                      kubectl create/update Secret in <target_k8s_ns>
    │                      POST /k8s/mapping/status  { status: "synced" | "error" }
    │
    ▼
sleep SYNC_INTERVAL
    │
    └──► repeat
```

**Status values:**

| Status | Meaning |
|---|---|
| `pending` | Mapping was just created, operator has not synced yet |
| `synced` | Last sync completed successfully |
| `error` | Last sync failed — check operator logs for details |

---

## Troubleshooting

### Operator pod is `CrashLoopBackOff`

```bash
kubectl logs -n rune-system -l app=rune-operator
```

Common causes:
- `RUNE_SERVER` or `RUNE_TOKEN` env vars are missing or wrong — check the `rune-operator-credentials` Secret.
- The Rune server is not reachable from inside the cluster — verify the URL and that the vault is unsealed.

### Status stuck on `pending`

The operator has not completed its first poll. Check:
```bash
kubectl logs -n rune-system -l app=rune-operator --tail=50
```

If you see `connection refused`, the operator cannot reach the Rune server. On macOS with Docker Desktop, make sure you used `host.docker.internal` (not `localhost` or `172.x.x.x`).

### Secrets are not updating after a Rune secret changes

The operator re-syncs on the `SYNC_INTERVAL` clock. Wait for the next cycle, or restart the operator to force an immediate sync:

```bash
kubectl rollout restart deploy/rune-operator -n rune-system
```

### `error: mapping_id not found` in logs

The mapping was deleted in Rune while the operator was mid-cycle. This is harmless — it clears on the next cycle.

### Vault is sealed — operator stopped syncing

Wildcard system tokens do not support sealed-vault access. Unseal the vault and the operator will resume on its next poll automatically.

---

## Updating the operator token

If you rotate or revoke the system token:

1. Create a new Kubernetes system token in the Rune UI (Tokens → System Token → Kubernetes).
2. Update the credentials secret and restart the operator:

```bash
kubectl create secret generic rune-operator-credentials \
  --namespace rune-system \
  --from-literal=RUNE_SERVER="http://host.docker.internal:2710" \
  --from-literal=RUNE_TOKEN="<NEW_TOKEN>" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl rollout restart deploy/rune-operator -n rune-system
```

---

## Security notes

- The operator only has **read** access to Rune secrets — the system token is `read_only: true`.
- Rune never holds any Kubernetes credentials or communicates outbound to your cluster.
- K8s Secrets created by the operator are labeled `managed-by: rune-operator` — do not edit them manually; they will be overwritten on the next sync.
- The operator's ClusterRole grants secrets access cluster-wide. If you want to restrict this to specific namespaces, replace `ClusterRole` + `ClusterRoleBinding` with `Role` + `RoleBinding` scoped to the namespaces you use.
