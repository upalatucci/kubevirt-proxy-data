#!/bin/bash

# Local development script for kubevirt-apiserver-proxy

set -euo pipefail

export APP_ENV=dev

KUBE_API_SERVER=$(oc config view --minify -o jsonpath='{.clusters[0].cluster.server}' | sed 's|https://||')

if [ -z "$KUBE_API_SERVER" ]; then
  echo "Error: Failed to get API server from oc config. Make sure you're logged in."
  exit 1
fi

export KUBE_API_SERVER

echo "Using API Server: $KUBE_API_SERVER"

WATCH=true
if [[ "${1:-}" == "--once" ]]; then
  WATCH=false
fi

run_once() {
  go build -o ./kubevirt-apiserver-proxy .
  ./kubevirt-apiserver-proxy
}

run_with_reload() {
  AIR_BIN=""
  if command -v air >/dev/null 2>&1; then
    AIR_BIN="air"
  elif [[ -x "$(go env GOPATH)/bin/air" ]]; then
    AIR_BIN="$(go env GOPATH)/bin/air"
  else
    echo "Installing air for live reload..."
    go install github.com/air-verse/air@latest
    if [[ -x "$(go env GOPATH)/bin/air" ]]; then
      AIR_BIN="$(go env GOPATH)/bin/air"
    fi
  fi

  if [[ -z "$AIR_BIN" ]]; then
    echo "Error: air is not on PATH. Add \"\$(go env GOPATH)/bin\" to your PATH."
    echo "Falling back to one-shot mode. Use ./start-proxy.sh --once explicitly next time."
    run_once
    return
  fi

  echo "Watching Go files for changes (Ctrl+C to stop). Use --once to disable reload."
  "$AIR_BIN"
}

if [[ "$WATCH" == "true" ]]; then
  run_with_reload
else
  run_once
fi
