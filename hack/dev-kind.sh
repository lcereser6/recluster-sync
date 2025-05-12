#!/usr/bin/env bash
set -euo pipefail

# ───── config ──────────────────────────────────────────────────────────
REG=ghcr.io/lcereser6
CA_IMAGE="${REG}/cluster-autoscaler-arm64:dev"
RC_SYNC_IMAGE="${REG}/recluster-sync:dev"
KWOK_VER="v0.5.2"
KIND_VER="v1.30.0"
# ───────────────────────────────────────────────────────────────────────

echo "↻ Re-creating kind (${KIND_VER})"
kind delete cluster || true
kind create cluster --image "kindest/node:${KIND_VER}"

echo "↻ Installing KWOK ${KWOK_VER}"
curl -sL "https://github.com/kubernetes-sigs/kwok/releases/download/${KWOK_VER}/kwok.yaml"        | kubectl apply -f -
curl -sL "https://github.com/kubernetes-sigs/kwok/releases/download/${KWOK_VER}/stage-fast.yaml" | kubectl apply -f -

echo "↻ Loading dev images into kind"
kind load docker-image "${CA_IMAGE}"
kind load docker-image "${RC_SYNC_IMAGE}"

echo "↻ Applying Rcnode CRD"
kubectl apply -f $HOME/src/recluster-sync/config/crd/bases

echo "↻ Deploying Cluster-Autoscaler"
helm install ca $HOME/src/autoscaler-recluster/charts/cluster-autoscaler \
  --namespace kube-system --create-namespace \
  --set image.repository="$REG/cluster-autoscaler-arm64" \
  --set image.tag=dev \
  --set autoDiscovery.clusterName=kind \
  --set autoDiscovery.enabled=true \
  --set extraArgs.cluster-name=kind \
  --set extraArgs.cloud-provider=recluster

echo "↻ Deploying recluster-sync controller (KWOK mode)"
helm install sync $HOME/src/recluster-sync/charts \
  --namespace kube-system \
  --set image.repository="${REG}/recluster-sync" \
  --set image.tag=dev \
  --set image.mode=kwok   # ← key changed here