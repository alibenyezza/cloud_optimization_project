#!/usr/bin/env bash
# TD4 Part 4.1 — Cas de test du scheduler Round-Robin (aucune VM nécessaire).
#
# Usage : ./scripts/test_scheduler.sh   (après avoir compilé bin/round_robin_scheduler)
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin/round_robin_scheduler"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [ ! -x "$BIN" ]; then
    echo "error: $BIN not found, build it first" >&2
    exit 1
fi

PASS=0
FAIL=0
ok()   { echo "  PASS  $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL  $1"; FAIL=$((FAIL + 1)); }

# make_tasks N [ID...] : N tâches T1..TN, ou les IDs donnés.
make_tasks() {
    echo "task_id,workload,size"
    if [ "$#" -gt 1 ]; then
        shift
        for id in "$@"; do echo "$id,cpu_task,1000"; done
    else
        for i in $(seq 1 "$1"); do echo "T$i,cpu_task,1000"; done
    fi
}
make_machines() {
    echo "vm"
    for i in $(seq 1 "$1"); do echo "vm$i"; done
}
# counts FILE : "vm1=2 vm2=2 vm3=2" (VM sans tâche absentes)
counts() {
    tail -n +2 "$1" | cut -d, -f4 | sort | uniq -c | awk '{printf "%s%s=%s", sep, $2, $1; sep=" "}'
}

echo "Case A: 6 tasks, 3 VMs -> 2 per VM"
make_tasks 6 > "$TMP/t.csv"; make_machines 3 > "$TMP/m.csv"
"$BIN" "$TMP/t.csv" "$TMP/m.csv" "$TMP/a.csv" > /dev/null
[ "$(counts "$TMP/a.csv")" = "vm1=2 vm2=2 vm3=2" ] && ok "$(counts "$TMP/a.csv")" || fail "$(counts "$TMP/a.csv")"

echo "Case B: 5 tasks, 2 VMs -> alternate, 3 and 2"
make_tasks 5 > "$TMP/t.csv"; make_machines 2 > "$TMP/m.csv"
"$BIN" "$TMP/t.csv" "$TMP/m.csv" "$TMP/a.csv" > /dev/null
ORDER="$(tail -n +2 "$TMP/a.csv" | cut -d, -f4 | paste -sd' ')"
[ "$ORDER" = "vm1 vm2 vm1 vm2 vm1" ] && ok "order: $ORDER" || fail "order: $ORDER"

echo "Case C: 2 tasks, 3 VMs -> vm1 and vm2 get one, vm3 none"
make_tasks 2 > "$TMP/t.csv"; make_machines 3 > "$TMP/m.csv"
"$BIN" "$TMP/t.csv" "$TMP/m.csv" "$TMP/a.csv" > /dev/null
[ "$(counts "$TMP/a.csv")" = "vm1=1 vm2=1" ] && ok "$(counts "$TMP/a.csv")" || fail "$(counts "$TMP/a.csv")"

echo "Case D: 5 tasks, 0 VMs -> error, no allocation written"
make_tasks 5 > "$TMP/t.csv"; make_machines 0 > "$TMP/m.csv"; rm -f "$TMP/a.csv"
if MSG="$("$BIN" "$TMP/t.csv" "$TMP/m.csv" "$TMP/a.csv" 2>&1)"; then
    fail "exit code 0"
elif [ -e "$TMP/a.csv" ]; then
    fail "allocation file was created"
else
    ok "$MSG"
fi

echo "Case E: duplicate task IDs -> rejected"
make_tasks 3 T1 T2 T1 > "$TMP/t.csv"; make_machines 3 > "$TMP/m.csv"; rm -f "$TMP/a.csv"
if MSG="$("$BIN" "$TMP/t.csv" "$TMP/m.csv" "$TMP/a.csv" 2>&1)"; then
    fail "exit code 0"
else
    ok "$MSG"
fi

echo "Case F (question): reversed task order changes the assignments"
make_tasks 3 T1 T2 T3 > "$TMP/t.csv"; make_machines 3 > "$TMP/m.csv"
"$BIN" "$TMP/t.csv" "$TMP/m.csv" "$TMP/a1.csv" > /dev/null
make_tasks 3 T3 T2 T1 > "$TMP/t.csv"
"$BIN" "$TMP/t.csv" "$TMP/m.csv" "$TMP/a2.csv" > /dev/null
T1_BEFORE="$(grep '^T1,' "$TMP/a1.csv" | cut -d, -f4)"
T1_AFTER="$(grep '^T1,' "$TMP/a2.csv" | cut -d, -f4)"
[ "$T1_BEFORE" != "$T1_AFTER" ] && ok "T1: $T1_BEFORE -> $T1_AFTER" || fail "T1 stayed on $T1_BEFORE"

echo "Determinism: same inputs -> identical output"
"$BIN" "$TMP/t.csv" "$TMP/m.csv" "$TMP/a3.csv" > /dev/null
cmp -s "$TMP/a2.csv" "$TMP/a3.csv" && ok "identical" || fail "outputs differ"

echo
echo "$PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
