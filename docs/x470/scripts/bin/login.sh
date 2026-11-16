#!/bin/bash
P=/tmp/claude-1000/-opt-sparked-asrockrack-api/4c303595-4244-4c22-8dc9-6968bfcb1de5/scratchpad/probe
set -a; . /opt/sparked/asrockrack-api/bmcprobe.env; set +a; umask 077
curl -sk -c $P/cjar -D $P/raw/api_login.hdr -o $P/raw/api_login.json -w 'login %{http_code} %{time_total}\n' -H 'X-Requested-With: XMLHttpRequest' -H 'Accept: application/json' --data @$P/loginform https://$BMC_HOST/api/session
node -e 'try{process.stdout.write(require(process.argv[1]).CSRFToken||"")}catch(e){}' $P/raw/api_login.json > $P/csrf
node -e 'const j=require(process.argv[1]);console.log("racsession_id",j.racsession_id)' $P/raw/api_login.json
