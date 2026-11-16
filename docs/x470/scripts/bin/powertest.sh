#!/bin/bash
# One legacy power-on then off cycle. Host must start OFF; leaves host OFF.
P=/tmp/claude-1000/-opt-sparked-asrockrack-api/4c303595-4244-4c22-8dc9-6968bfcb1de5/scratchpad/probe
OUT=$P/raw/powertest.txt; : > $OUT
now() { date +%s.%N; }
st() { $P/bin/api.sh GET /api/chassis pw_poll >/dev/null; grep -o '"power_status": [0-9]' $P/raw/api/pw_poll.json | awk '{print $2}'; }
s=$(st); echo "initial power_status=$s" >> $OUT
[ "$s" = "0" ] || { echo "ABORT host not off" >> $OUT; exit 1; }
t0=$(now); echo "$($P/bin/api.sh POST /api/actions/power pw_on '{"power_command":1}') body=$(cat $P/raw/api/pw_on.json)" >> $OUT
for i in $(seq 1 40); do s=$(st); if [ "$s" = "1" ]; then echo "on after $(echo "$(now)-$t0" | bc) s (poll $i)" >> $OUT; break; fi; sleep 0.5; done
sleep 20
$P/bin/api.sh GET /api/sensors pw_sensors_on >> $OUT
t1=$(now); echo "$($P/bin/api.sh POST /api/actions/power pw_soft '{"power_command":5}') body=$(cat $P/raw/api/pw_soft.json)" >> $OUT
off=0
for i in $(seq 1 60); do s=$(st); if [ "$s" = "0" ]; then echo "off after soft in $(echo "$(now)-$t1" | bc) s" >> $OUT; off=1; break; fi; sleep 1; done
if [ $off = 0 ]; then
  t2=$(now); echo "soft-off did not complete in ~60 polls; $($P/bin/api.sh POST /api/actions/power pw_hard '{"power_command":0}') body=$(cat $P/raw/api/pw_hard.json)" >> $OUT
  for i in $(seq 1 40); do s=$(st); if [ "$s" = "0" ]; then echo "off after hard in $(echo "$(now)-$t2" | bc) s" >> $OUT; break; fi; sleep 0.5; done
fi
echo "final power_status=$(st)" >> $OUT
echo DONE >> $OUT
