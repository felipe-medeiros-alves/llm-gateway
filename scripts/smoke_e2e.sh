#!/usr/bin/env bash
set -euo pipefail
BASE=${BASE:-http://localhost:8080}
KEY=dev-key

echo "== health =="
curl -sf "$BASE/healthz" >/dev/null

echo "== non-stream =="
curl -sf "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"smart-chat","messages":[{"role":"user","content":"ping"}]}' | tee /tmp/gw.json
grep -q 'mock' /tmp/gw.json

echo "== stream =="
curl -sfN "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"smart-chat","stream":true,"messages":[{"role":"user","content":"ping"}]}' | head -5

echo "== cache hit =="
curl -sf "$BASE/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"smart-chat","messages":[{"role":"user","content":"ping"}]}' | grep -q 'mock'

echo "== metrics =="
curl -sf "$BASE/metrics" | grep -q gateway_requests_total

echo "smoke ok"
