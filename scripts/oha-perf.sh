#!/usr/bin/env bash
# Load-test the ledger HTTP API with oha and keep a record of every run.
#
#   ./scripts/oha-perf.sh                    # all scenarios
#   ./scripts/oha-perf.sh balance-hot        # one scenario
#   DURATION=60s CONC=500 ./scripts/oha-perf.sh balance-spread
#   ./scripts/oha-perf.sh --list
#
# Assumes the ledger is running (go run ./services/ledger) against seeded data
# (./scripts/db-reset.sh). Results go to perf-results/<timestamp>-<scenario>.txt
#
# Note: oha sends the same headers and body on every request, so it can't give
# each transfer a fresh Idempotency-Key. The transfer scenario below therefore
# measures the *replay* path. Unique transfers need a tool that varies requests
# (vegeta with a generated targets file, or a small Go load generator).
set -euo pipefail
cd "$(dirname "$0")/.."

BASE_URL="${BASE_URL:-http://localhost:8080}"
DURATION="${DURATION:-30s}"
CONC="${CONC:-200}"
RESULTS_DIR="perf-results"

# Seed data (see services/ledger/seeds): accounts ...8002-000000000001 to ...013
ACCT_PREFIX="00000000-0000-7000-8002-0000000000"
ACCT_A="${ACCT_PREFIX}01"
ACCT_B="${ACCT_PREFIX}02"
CUSTOMER="00000000-0000-7000-8001-000000000001"

# ---------------------------------------------------------------- scenarios --

scenario_balance_hot() {
  # Every request reads the same account: best case, everything cached.
  oha -z "$DURATION" -c "$CONC" --no-tui \
    "$BASE_URL/accounts/$ACCT_A/balance"
}

scenario_balance_spread() {
  # Random account out of the 13 seeded ones per request.
  oha -z "$DURATION" -c "$CONC" --no-tui --rand-regex-url \
    "$BASE_URL/accounts/${ACCT_PREFIX}(0[1-9]|1[0-3])/balance"
}

scenario_balance_404() {
  # Unknown account: measures the not-found path.
  oha -z "$DURATION" -c "$CONC" --no-tui \
    "$BASE_URL/accounts/00000000-0000-7000-8002-999999999999/balance"
}

scenario_transfer_replay() {
  # One key for every request: the first moves 1p, the rest are replays
  # (200 + Idempotent-Replayed). Measures the idempotency lookup path.
  local key="perf-replay-$(date +%s)"
  oha -z "$DURATION" -c "$CONC" --no-tui -m POST \
    -H "Content-Type: application/json" \
    -H "Idempotency-Key: $key" \
    -H "X-Customer-Id: $CUSTOMER" \
    -d "{\"from_account_id\":\"$ACCT_A\",\"to_account_id\":\"$ACCT_B\",\"amount_minor\":1,\"currency\":\"GBP\"}" \
    "$BASE_URL/transfers"
}

SCENARIOS=(balance-hot balance-spread balance-404 transfer-replay)

# ------------------------------------------------------------------ helpers --

usage() {
  echo "usage: $0 [--list] [scenario ...]"
  echo "scenarios: ${SCENARIOS[*]}"
  echo "env: BASE_URL=$BASE_URL DURATION=$DURATION CONC=$CONC"
}

require() {
  command -v oha >/dev/null || { echo "oha not found: cargo install oha"; exit 1; }
  if ! curl -sf -o /dev/null "$BASE_URL/accounts/$ACCT_A/balance"; then
    echo "ledger not reachable at $BASE_URL (is it running and seeded?)"
    exit 1
  fi
}

# Pull the headline numbers out of oha's text report.
summarise() {
  local file="$1" name="$2"
  local rps p50 p99 codes aborted
  rps=$(grep -m1 'Requests/sec:' "$file" | awk '{printf "%.0f", $2}')
  p50=$(to_ms "$(grep -m1 '50.00% in' "$file")")
  p99=$(to_ms "$(grep -m1 '99.00% in' "$file")")
  codes=$(grep -E '^\s*\[[0-9]{3}\] [0-9]+ responses' "$file" \
            | awk '{printf "%s=%s ", $1, $2}' | tr -d '[]')
  aborted=$(grep -E 'aborted' "$file" | grep -oE '\[[0-9]+\]' | tr -d '[]' | paste -sd+ | bc 2>/dev/null)
  printf "%-18s %10s rps   p50 %8s ms   p99 %8s ms   %s%s\n" \
    "$name" "${rps:-?}" "${p50:-?}" "${p99:-?}" "${codes:-?}" \
    "${aborted:+  (aborted at deadline: $aborted)}"
}

to_ms() {
  awk '{
    v = $3; u = $4
    if (u ~ /^s/)        v *= 1000
    else if (u ~ /^(µs|us)/) v /= 1000
    else if (u ~ /^ns/)  v /= 1000000
    printf "%.2f", v
  }' <<<"$1"
}

run_scenario() {
  local name="$1"
  local fn="scenario_${name//-/_}"
  declare -F "$fn" >/dev/null || { echo "unknown scenario: $name"; usage; exit 1; }

  local out="$RESULTS_DIR/$STAMP-$name.txt"
  echo "==> $name  (duration $DURATION, $CONC connections)"
  {
    echo "# scenario=$name duration=$DURATION conc=$CONC base=$BASE_URL"
    echo "# git=$(git rev-parse --short HEAD 2>/dev/null || echo none) host=$(hostname)"
    "$fn"
  } >"$out" 2>&1
  SUMMARY+=("$(summarise "$out" "$name")")
}

# --------------------------------------------------------------------- main --

if [[ "${1:-}" == "--list" || "${1:-}" == "-h" ]]; then usage; exit 0; fi

require
mkdir -p "$RESULTS_DIR"
STAMP="$(date +%Y%m%d-%H%M%S)"
SUMMARY=()

if [[ $# -eq 0 ]]; then set -- "${SCENARIOS[@]}"; fi
for s in "$@"; do run_scenario "$s"; done

echo
echo "==> summary ($STAMP)"
printf '%s\n' "${SUMMARY[@]}"
echo "full reports: $RESULTS_DIR/$STAMP-*.txt"