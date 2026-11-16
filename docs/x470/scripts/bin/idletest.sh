#!/bin/bash
# Measures legacy /api web session idle timeout. Each probe resets the idle timer if the session is alive.
P=/tmp/claude-1000/-opt-sparked-asrockrack-api/4c303595-4244-4c22-8dc9-6968bfcb1de5/scratchpad/probe
OUT=$P/raw/idletest.txt
: > $OUT
$P/bin/login.sh >> $OUT
for g in 30 60 90 120 150 200 300; do
  sleep $g
  r=$($P/bin/api.sh GET /api/chassis idle_$g)
  echo "$(date -u +%T) gap=${g}s -> $r" >> $OUT
  if ! echo "$r" | grep -q ' 200 '; then
    $P/bin/login.sh >> $OUT
  fi
done
$P/bin/api.sh DELETE /api/session idle_logout >> $OUT
echo DONE >> $OUT
