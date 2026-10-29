#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

for command in go npm lsof; do
  if ! command -v "$command" >/dev/null; then
    printf 'Required command is missing: %s\n' "$command" >&2
    exit 1
  fi
done

# Check both ports before starting anything. Never replace a running service.
for port in 8091 5174; do
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    printf 'Port %s is already in use; leaving the running services untouched.\n' "$port" >&2
    exit 1
  fi
done
if [[ ! -x frontend/node_modules/.bin/next ]]; then
  printf 'Frontend dependencies are missing. Run make install first.\n' >&2
  exit 1
fi

# Separate process groups let cleanup include go-run/npm descendants.
set -m
backend_pid=''
web_pid=''
cleanup() {
  trap - EXIT INT TERM
  for pid in "$backend_pid" "$web_pid"; do
    if [[ -n "$pid" ]]; then
      kill -TERM -- "-$pid" 2>/dev/null || true
    fi
  done
  for pid in "$backend_pid" "$web_pid"; do
    if [[ -n "$pid" ]]; then
      wait "$pid" 2>/dev/null || true
    fi
  done
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

(cd backend && exec go run . serve --http=127.0.0.1:8091) &
backend_pid=$!
(cd frontend && NEXT_PUBLIC_PB_URL=http://127.0.0.1:8091 exec npm run dev) &
web_pid=$!
printf 'App: http://127.0.0.1:5174 (API on http://127.0.0.1:8091)\nCtrl+C stops both services started here.\n'

# macOS ships Bash 3.2, which does not support wait -n.
while kill -0 "$backend_pid" 2>/dev/null && kill -0 "$web_pid" 2>/dev/null; do
  sleep 1
done
if ! kill -0 "$backend_pid" 2>/dev/null; then
  wait "$backend_pid"
else
  wait "$web_pid"
fi
