#!/usr/bin/env bash
# rune-operator setup — creates the namespace, credentials secret, and deploys the operator
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo ""
echo "  rune-operator setup"
echo "  ───────────────────────────────────────────"
echo ""

# ── Collect inputs ────────────────────────────────────────────────────────────
read -rp "  Rune server URL  (e.g. http://10.0.0.1:2710): " RUNE_SERVER
if [[ -z "$RUNE_SERVER" ]]; then
  echo "  ERROR: server URL cannot be empty" >&2
  exit 1
fi

read -rsp "  Rune token       (rnk_ or stk_ — input hidden): " RUNE_TOKEN
echo ""
if [[ -z "$RUNE_TOKEN" ]]; then
  echo "  ERROR: token cannot be empty" >&2
  exit 1
fi

echo ""

# ── Verify kubectl is available ───────────────────────────────────────────────
if ! command -v kubectl &>/dev/null; then
  echo "  ERROR: kubectl not found — install it and configure your cluster context first" >&2
  exit 1
fi

echo "  Using cluster: $(kubectl config current-context)"
echo ""

# ── Create namespace ──────────────────────────────────────────────────────────
echo "  [1/3] Creating namespace rune-system..."
kubectl create namespace rune-system --dry-run=client -o yaml | kubectl apply -f -

# ── Create credentials secret ─────────────────────────────────────────────────
echo "  [2/3] Creating credentials secret..."
kubectl create secret generic rune-operator-credentials \
  --namespace rune-system \
  --from-literal=RUNE_SERVER="$RUNE_SERVER" \
  --from-literal=RUNE_TOKEN="$RUNE_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -

# ── Deploy operator ───────────────────────────────────────────────────────────
echo "  [3/3] Deploying rune-operator..."
kubectl apply -f "$SCRIPT_DIR/deploy.yaml"

# ── Wait for rollout ──────────────────────────────────────────────────────────
echo ""
echo "  Waiting for rollout..."
kubectl rollout status deployment/rune-operator -n rune-system --timeout=120s

echo ""
echo "  ✓ rune-operator is running"
echo ""
echo "  Logs:"
echo "    kubectl logs -n rune-system -l app=rune-operator --follow"
echo ""
echo "  Create a mapping via the Rune UI (Kubernetes view) or API:"
echo "    POST $RUNE_SERVER/k8s/mapping/create"
echo ""
