#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

GO_CMD=${1:-go}
CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
REPO_ROOT="$(git rev-parse --show-toplevel)"
CODEGEN_PKG=$($GO_CMD list -m -mod=readonly -f "{{.Dir}}" k8s.io/code-generator)
cd "${CURRENT_DIR}/.."

echo "Running code generation for recluster-sync..."

source "${CODEGEN_PKG}/kube_codegen.sh"

echo "Generating helpers for RCNODES AND RCPOLICIES CRD..."

kube::codegen::gen_helpers \
    --boilerplate hack/boilerplate.go.txt \
    "${REPO_ROOT}/apis/recluster.com/v1alpha1"

echo "Generating client code for RCNODES AND RCPOLICIES CRD..."

kube::codegen::gen_client \
    --output-pkg github.com/lcereser6/recluster-sync/apis/client \
    --output-dir "${REPO_ROOT}/apis/client" \
    --boilerplate "${REPO_ROOT}/hack/boilerplate.go.txt" \
    --with-watch \
    --with-applyconfig \
    "${REPO_ROOT}/apis"


echo "Generated client code, running go mod tidy..."
"${GO_CMD}" mod tidy
