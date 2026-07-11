#!/usr/bin/env bash
#
# smoke.sh — end-to-end CLI smoke test for the tpty game engine.
#
# It builds ./cmd/tpty and drives the REAL binary through the whole MVP loop
# against a throwaway data directory: create → world → starting provinces →
# player → advance (0→1) → report → submit orders → process → advance (1→2) →
# report. It prints each step's command and output and ASSERTS the key outcomes:
#
#   * the turn counter reaches 2 (0 → 1 → 2);
#   * a turn report file exists; and
#   * the seeded entity's location differs between the turn-1 and turn-2 reports
#     (reports reflect start-of-turn state: the turn-1 report shows the entity at
#     its starting province, the turn-2 report at the moved province).
#
# The submitted orders exercise a real handler and the stub no-op: "move 2" (NE)
# and "hold" execute for real; "work" is recorded as a stub. Any failure exits
# non-zero with a clear message.
#
# Usage:
#   ./scripts/smoke.sh [data-dir]
#
# data-dir defaults to games/claude (git-ignored). It is wiped clean at the
# start and removed on success, so the script is safe to re-run. Override the
# directory with the first argument or the SMOKE_DATA environment variable.

set -euo pipefail

# --- locate the repo root (this script lives in scripts/) -------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

DATA_DIR="${1:-${SMOKE_DATA:-games/claude}}"
GAME_ID="smoke-game"
BIN="$(mktemp -d)/tpty"

fail() {
	echo "SMOKE FAILED: $*" >&2
	exit 1
}

step() {
	echo
	echo "\$ $*"
	"$@"
}

# --- clean slate ------------------------------------------------------------
echo "smoke: repo root  = ${REPO_ROOT}"
echo "smoke: data dir   = ${DATA_DIR}"
rm -rf "${DATA_DIR}"
mkdir -p "${DATA_DIR}"

# --- build the real binary --------------------------------------------------
echo
echo "\$ go build -o ${BIN} ./cmd/tpty"
go build -o "${BIN}" ./cmd/tpty || fail "build failed"

# tpty reads TPTY_* env vars; pass --data explicitly so a stray TPTY_DATA in the
# environment cannot redirect us. TPTY_ENV=development keeps dotenv predictable.
export TPTY_ENV=development
tpty() { "${BIN}" --data "${DATA_DIR}" "$@"; }

# --- the loop ---------------------------------------------------------------
step tpty game create --game-id "${GAME_ID}" --seed1 1 --seed2 2

step tpty world generate --rings 3

# Ring 1 default set includes (1,-1); the player starts there.
step tpty world starting-provinces generate --ring 1

START_PROVINCE="(1,-1)"
step tpty player create --email "smoke@example.com" --handle "smoker" \
	--starting-province "${START_PROVINCE}"

# advance 0→1 seeds a faction and a starting entity for each active player.
step tpty turn advance

# report at turn 1 — before processing, so it shows the STARTING province.
step tpty turn report

# --- read the player's password and id out of players.json ------------------
read_player() {
	python3 - "${DATA_DIR}/players.json" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    store = json.load(f)
p = store["players"][0]
print(p["id"])
print(p["password"])
PY
}
PLAYER_ID="$(read_player | sed -n 1p)"
PLAYER_PW="$(read_player | sed -n 2p)"
[ -n "${PLAYER_ID}" ] || fail "could not read player id from players.json"
[ -n "${PLAYER_PW}" ] || fail "could not read player password from players.json"
echo
echo "smoke: player id  = ${PLAYER_ID}"

# --- build and submit an orders file (real + stub commands) -----------------
ORDERS_FILE="${DATA_DIR}/orders.txt"
cat > "${ORDERS_FILE}" <<EOF
"${GAME_ID}" ${PLAYER_ID} "${PLAYER_PW}"

entity 1, "Entity 1"
    move 2
    hold
    work
EOF
echo
echo "smoke: orders file ${ORDERS_FILE}:"
sed 's/^/    /' "${ORDERS_FILE}"

step tpty orders submit --file "${ORDERS_FILE}"
step tpty orders list

# process turn 1: applies move + hold, records work as a stub.
step tpty turn process

# advance 1→2: commits the processed turn (no new seeding).
step tpty turn advance

# report at turn 2 — now reflects the MOVED province.
step tpty turn report

# --- assertions -------------------------------------------------------------
echo
echo "smoke: asserting outcomes"

# the turn counter reached 2.
GAME_TURN="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["turn"])' "${DATA_DIR}/game.json")"
[ "${GAME_TURN}" = "2" ] || fail "game.turn = ${GAME_TURN}, want 2"
echo "  ok: game.turn reached ${GAME_TURN} (0 → 1 → 2)"

# a turn report file exists for turn 2.
REPORT1="$(printf '%s/reports/turn-0001/player-%04d.json' "${DATA_DIR}" "${PLAYER_ID}")"
REPORT2="$(printf '%s/reports/turn-0002/player-%04d.json' "${DATA_DIR}" "${PLAYER_ID}")"
[ -f "${REPORT1}" ] || fail "turn-1 report ${REPORT1} does not exist"
[ -f "${REPORT2}" ] || fail "turn-2 report ${REPORT2} does not exist"
echo "  ok: turn report files exist"

# the entity location differs between the turn-1 and turn-2 reports.
report_loc() {
	python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["entities"][0]["location"])' "$1"
}
LOC1="$(report_loc "${REPORT1}")"
LOC2="$(report_loc "${REPORT2}")"
echo "  turn-1 report entity location = ${LOC1}"
echo "  turn-2 report entity location = ${LOC2}"
[ "${LOC1}" = "${START_PROVINCE}" ] || fail "turn-1 report location = ${LOC1}, want ${START_PROVINCE}"
[ "${LOC2}" = "(2,-2)" ] || fail "turn-2 report location = ${LOC2}, want (2,-2) (NE of start)"
[ "${LOC1}" != "${LOC2}" ] || fail "turn-1 and turn-2 report locations both ${LOC1}; reports must reflect start-of-turn state"
echo "  ok: entity moved ${LOC1} → ${LOC2} between reports"

# --- clean up ---------------------------------------------------------------
rm -rf "${DATA_DIR}"
rm -rf "$(dirname "${BIN}")"

echo
echo "SMOKE PASSED"
