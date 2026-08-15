#!/usr/bin/env bash

set -uo pipefail

api_pid=""
frontend_pid=""

cleanup() {
  trap - EXIT INT TERM
  for pid in "$api_pid" "$frontend_pid"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill -TERM "$pid" 2>/dev/null || true
    fi
  done
  for pid in "$api_pid" "$frontend_pid"; do
    if [[ -n "$pid" ]]; then
      wait "$pid" 2>/dev/null || true
    fi
  done
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

make --no-print-directory run-api &
api_pid=$!
make --no-print-directory run-frontend &
frontend_pid=$!

wait -n "$api_pid" "$frontend_pid"
