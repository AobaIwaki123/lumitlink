#!/usr/bin/env bash

set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_DIR"

echo "1. Checking formatting"
if [[ -n "$(gofmt -l cmd internal)" ]]; then
  echo "gofmt required"
  gofmt -l cmd internal
  exit 1
fi

echo "2. Running go vet"
go vet ./...

echo "3. Running tests with race detector"
go test -race -cover ./...

echo "4. Building binary"
go build -trimpath -o /tmp/lumipage-verify ./cmd/lumipage

echo "5. Rendering Kubernetes manifests"
kubectl kustomize k8s/manifests >/tmp/lumipage-k8s.yaml
test -s /tmp/lumipage-k8s.yaml

echo "All local verification checks passed."
