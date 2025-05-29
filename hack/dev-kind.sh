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
helm repo add kwok https://kwok.sigs.k8s.io/charts/
helm upgrade --namespace kube-system --install kwok1 kwok/kwok
helm upgrade --install kwok2 kwok/stage-fast
helm upgrade --install kwok3 kwok/metrics-usage


echo "↻ Loading dev images into kind"
kind load docker-image "${CA_IMAGE}"
kind load docker-image "${RC_SYNC_IMAGE}"

echo "↻ Applying Rcnode CRD"
kubectl apply -f $HOME/src/recluster-sync/config/crd/bases

kubectl apply -f $HOME/src/recluster-sync/resources/rcnodes.yaml   



echo "↻ Deploying Cluster-Autoscaler"
helm install ca $HOME/src/autoscaler-recluster/charts/cluster-autoscaler \
  --namespace kube-system --create-namespace \
  --set image.repository="$REG/cluster-autoscaler-arm64" \
  --set image.tag=dev \
  --set autoDiscovery.clusterName=kind \
  --set autoDiscovery.enabled=true \
  --set extraArgs.cluster-name=kind \
  --set cloudProvider=recluster \
  --set scale-down-enabled=true \
  --set scale-down-unneeded-time=30s \
  --set scale-down-delay-after-add=30s \
  --set scale-down-delay-after-failure=30s \

echo "↻ Deploying recluster-sync controller (KWOK mode)"
helm install sync $HOME/src/recluster-sync/charts \
  --namespace kube-system \
  --set image.repository="${REG}/recluster-sync" \
  --set image.tag=dev \
  --set image.mode=kwok   # ← key changed here
