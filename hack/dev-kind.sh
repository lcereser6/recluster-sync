#!/usr/bin/env bash
set -euo pipefail

REG=ghcr.io/lcereser6
CA_IMAGE="${REG}/cluster-autoscaler-arm64:dev"
RC_SYNC_IMAGE="${REG}/recluster-sync:dev"
KWOK_VER="v0.5.2"
KIND_VER="v1.30.0"
NS="kube-system"
RELEASE="sync"
CHART="$HOME/src/recluster-sync/charts"

echo "↻ Re-creating kind (${KIND_VER})"
kind delete cluster || true
kind create cluster --image "kindest/node:${KIND_VER}"

echo "↻ Installing cert-manager"
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm upgrade --install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true

echo "↻ Installing KWOK ${KWOK_VER}"
helm repo add kwok https://kwok.sigs.k8s.io/charts/
helm upgrade --namespace kube-system --install kwok1 kwok/kwok
helm upgrade --install kwok2 kwok/stage-fast
helm upgrade --install kwok3 kwok/metrics-usage

echo "↻ Loading dev images into kind"
kind load docker-image "${CA_IMAGE}"
kind load docker-image "${RC_SYNC_IMAGE}"

echo "↻ Applying RcNode CRD"
kubectl apply -f "$HOME/src/recluster-sync/config/crd/bases"

echo "↻ Deploying Cluster-Autoscaler"
helm upgrade --install ca "$HOME/src/autoscaler-recluster/charts/cluster-autoscaler" \
  --namespace "$NS" --create-namespace \
  --set image.repository="$REG/cluster-autoscaler-arm64" \
  --set image.tag=dev \
  --set autoDiscovery.clusterName=kind \
  --set autoDiscovery.enabled=true \
  --set extraArgs.cluster-name=kind \
  --set cloudProvider=recluster \
  --set scale-down-enabled=true \
  --set scale-down-unneeded-time=30s \
  --set scale-down-delay-after-add=30s \
  --set scale-down-delay-after-failure=30s

echo "↻ Phase 1: Deploy recluster-sync (Service/Deployment/Certificate), no MWC yet"
helm upgrade --install "$RELEASE" "$CHART" \
  --namespace "$NS" --create-namespace \
  --set image.repository="${REG}/recluster-sync" \
  --set image.tag=dev \
  --set image.mode=kwok \
  --set webhook.enabled=true \
  --set webhook.createWebhook=true \
  --set webhook.serviceName=recluster-sync-webhook \
  --set webhook.port=443 \
  --set webhook.targetPort=9443 \
  --set webhook.tls.secretName=recluster-sync-webhook-tls \
  --set webhook.tls.certPath=/certs \
  --set webhook.tls.certName=tls.crt \
  --set webhook.tls.keyName=tls.key

echo "↻ Waiting for webhook TLS Secret to be ready"
kubectl -n "$NS" rollout status deploy/"$(kubectl -n "$NS" get deploy -l app.kubernetes.io/instance=$RELEASE -o jsonpath='{.items[0].metadata.name}')" --timeout=120s || true

# Wait until cert-manager issues the secret
for i in {1..60}; do
  if kubectl -n "$NS" get secret recluster-sync-webhook-tls >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

echo "↻ Extracting CA bundle from TLS Secret"
CABUNDLE="$(kubectl -n "$NS" get secret recluster-sync-webhook-tls -o jsonpath='{.data.ca\.crt}')"
if [[ -z "${CABUNDLE}" ]]; then
  echo "ERROR: ca.crt is empty in secret recluster-sync-webhook-tls"
  exit 1
fi

echo "↻ Phase 2: Enable MutatingWebhookConfiguration with caBundle"
helm upgrade --install "$RELEASE" "$CHART" \
  --namespace "$NS" \
  --set image.repository="${REG}/recluster-sync" \
  --set image.tag=dev \
  --set image.mode=kwok \
  --set webhook.enabled=true \
  --set webhook.createWebhook=true \
  --set webhook.serviceName=recluster-sync-webhook \
  --set webhook.port=443 \
  --set webhook.targetPort=9443 \
  --set webhook.tls.secretName=recluster-sync-webhook-tls \
  --set webhook.tls.certPath=/certs \
  --set webhook.tls.certName=tls.crt \
  --set webhook.tls.keyName=tls.key \


echo "✅ Webhook wired. Quick dry-run mutation check:"
kubectl run test-gate --image=busybox --restart=Never --command -- echo ok \
  --dry-run=server -o yaml | yq '.spec.schedulingGates'