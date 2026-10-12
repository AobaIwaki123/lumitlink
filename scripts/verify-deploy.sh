#!/usr/bin/env bash

set -euo pipefail

NAMESPACE="lumipage"
DEPLOYMENT="lumipage"
INGRESS_HOST="lumipage.aooba.net"

echo "Deployment"
kubectl rollout status "deployment/${DEPLOYMENT}" -n "${NAMESPACE}" --timeout=120s
kubectl get deployment,pods,service,ingress -n "${NAMESPACE}" -o wide

echo "Health through the service"
kubectl run "lumipage-healthcheck-$(date +%s)" -n "${NAMESPACE}" --rm -i --restart=Never \
  --image=curlimages/curl:8.10.1 -- curl -fsS "http://${DEPLOYMENT}:8080/healthz"

echo "Ingress"
if curl -fsS --max-time 15 "https://${INGRESS_HOST}/healthz"; then
  echo
else
  echo "Ingress is not reachable yet; check Cloudflare tunnel and DNS."
fi

echo "Live logs: kubectl logs -f deployment/${DEPLOYMENT} -n ${NAMESPACE}"
