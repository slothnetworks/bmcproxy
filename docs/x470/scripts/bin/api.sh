#!/bin/bash
# usage: api.sh METHOD PATH OUTNAME [data]
P=/tmp/claude-1000/-opt-sparked-asrockrack-api/4c303595-4244-4c22-8dc9-6968bfcb1de5/scratchpad/probe
set -a; . /opt/sparked/asrockrack-api/bmcprobe.env; set +a
m=$1; p=$2; o=$3; d=$4
args=(-sk -b $P/cjar -X $m -H "X-CSRFTOKEN: $(cat $P/csrf)" -H 'X-Requested-With: XMLHttpRequest' -H 'Accept: application/json' -D $P/raw/api/$o.hdr -o $P/raw/api/$o.json -w "%{http_code} %{time_total} %{size_download}")
[ -n "$d" ] && args+=(-H 'Content-Type: application/json' --data "$d")
mkdir -p $P/raw/api
r=$(curl "${args[@]}" "https://$BMC_HOST$p")
echo "$m $p $r"
