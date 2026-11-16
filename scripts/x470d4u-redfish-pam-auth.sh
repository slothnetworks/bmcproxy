#!/usr/bin/env bash
#
# redfish-pam-auth.sh — enable/disable/inspect Redfish PAM authentication on an
# AMI MegaRAC / ASRock Rack BMC (e.g. X470D4U).
#
# WHY: On some ASRock boards (X470D4U firmware ~2.32/3.02) the Redfish service
# keeps its OWN account store and does NOT, by default, accept the IPMI/web
# ("/api") users such as `admin`. The board ships pam_ipmi.so wired into the
# Redfish PAM stack (/conf/pam_wounix) but gated behind the redis flag
# Redfish:AccountService:PAMEnabled, exposed as the writable OEM property
# PAMEnabled on /redfish/v1/AccountService/Configurations. Setting it true makes
# Redfish authenticate directly against the IPMI user DB via PAM — unifying the
# credential set without any user-sync daemon or firmware change.
#
# You must authenticate as a Redfish-capable admin to make the change. On a
# factory board that is the seeded `Administrator` account (default password is
# the AMI default). Supply it with --admin-user/--admin-pass; nothing is
# hardcoded here.
#
# The PATCH requires an If-Match ETag (the board returns 428 without one); this
# script fetches and supplies it automatically.
#
# Usage:
#   redfish-pam-auth.sh [options] <bmc-host>
#
# Options:
#   -a, --action {status|enable|disable}   default: status
#   -u, --admin-user USER                  Redfish admin (default: Administrator)
#   -p, --admin-pass PASS                  admin password (or env RF_ADMIN_PASS)
#   -t, --test-user USER                   after the change, verify this user can auth to Redfish
#   -T, --test-pass PASS                   password for --test-user (or env RF_TEST_PASS)
#       --port PORT                        HTTPS port (default: 443)
#       --timeout SECONDS                  per-request timeout (default: 25)
#   -h, --help
#
# Exit codes: 0 ok; 1 usage; 2 connectivity/HTTP; 3 auth failed; 4 resource missing.
#
# Examples:
#   RF_ADMIN_PASS=... redfish-pam-auth.sh -a status 10.3.10.207
#   RF_ADMIN_PASS=... redfish-pam-auth.sh -a enable -t admin -T adminpw 10.3.10.207
#
set -euo pipefail

ACTION=status
ADMIN_USER=Administrator
ADMIN_PASS=${RF_ADMIN_PASS:-}
TEST_USER=
TEST_PASS=${RF_TEST_PASS:-}
PORT=443
TIMEOUT=25
HOST=

die(){ echo "error: $*" >&2; exit "${2:-1}"; }

while [ $# -gt 0 ]; do
  case "$1" in
    -a|--action)     ACTION=${2:?}; shift 2;;
    -u|--admin-user) ADMIN_USER=${2:?}; shift 2;;
    -p|--admin-pass) ADMIN_PASS=${2:?}; shift 2;;
    -t|--test-user)  TEST_USER=${2:?}; shift 2;;
    -T|--test-pass)  TEST_PASS=${2:?}; shift 2;;
    --port)          PORT=${2:?}; shift 2;;
    --timeout)       TIMEOUT=${2:?}; shift 2;;
    -h|--help)       sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'; exit 0;;
    -*)              die "unknown option: $1";;
    *)               [ -z "$HOST" ] || die "unexpected arg: $1"; HOST=$1; shift;;
  esac
done

[ -n "$HOST" ] || die "missing <bmc-host> (try --help)" 1
case "$ACTION" in status|enable|disable) ;; *) die "bad --action: $ACTION" 1;; esac
[ -n "$ADMIN_PASS" ] || die "no admin password (use --admin-pass or RF_ADMIN_PASS)" 1
command -v curl >/dev/null || die "curl not found" 1
command -v python3 >/dev/null || die "python3 not found" 1

BASE="https://${HOST}:${PORT}"
CFG="/redfish/v1/AccountService/Configurations"
AUTH=(-u "${ADMIN_USER}:${ADMIN_PASS}")
CURL=(curl -sk --max-time "$TIMEOUT")

# low-level: print HTTP status for a quick probe
http_code(){ "${CURL[@]}" -o /dev/null -w '%{http_code}' "$@"; }

# reachability + root
root_code=$("${CURL[@]}" -o /dev/null -w '%{http_code}' "${BASE}/redfish/v1/" || true)
[ "$root_code" = "200" ] || die "cannot reach Redfish root at ${BASE}/redfish/v1/ (got '${root_code:-none}')" 2

# fetch Configurations (body + headers) as admin
tmph=$(mktemp); tmpb=$(mktemp); trap 'rm -f "$tmph" "$tmpb"' EXIT
code=$("${CURL[@]}" "${AUTH[@]}" -D "$tmph" -o "$tmpb" -w '%{http_code}' "${BASE}${CFG}" || true)
case "$code" in
  200) ;;
  401) die "admin auth rejected (401) for user '${ADMIN_USER}' — wrong password, or this account is not a Redfish user" 3;;
  404) die "${CFG} not found (404) — this BMC may not be an AMI/ASRock board with this OEM resource" 4;;
  *)   die "unexpected HTTP ${code} fetching ${CFG}" 2;;
esac

read_field(){ python3 -I -c "import sys,json;d=json.load(open('$tmpb'));print(d.get('$1'))" 2>/dev/null; }
CUR=$(read_field PAMEnabled)
ORDER=$(python3 -I -c "import json;print(json.load(open('$tmpb')).get('PAMOrder'))" 2>/dev/null)
ETAG=$(grep -i '^etag:' "$tmph" | tr -d '\r' | sed 's/^[Ee][Tt]ag: //' | tail -1)

echo "host:        ${HOST}:${PORT}"
echo "admin user:  ${ADMIN_USER}"
echo "PAMEnabled:  ${CUR}"
echo "PAMOrder:    ${ORDER}"

want=
case "$ACTION" in
  status)  exit 0;;
  enable)  want=true;;
  disable) want=false;;
esac

# already in desired state?
lc=$(printf '%s' "$CUR" | tr '[:upper:]' '[:lower:]')
if [ "$lc" = "$want" ]; then
  echo "already PAMEnabled=${want}; nothing to do."
else
  [ -n "$ETAG" ] || die "no ETag returned for ${CFG}; cannot do conditional PATCH" 2
  echo "patching PAMEnabled -> ${want} (If-Match: ${ETAG}) ..."
  pcode=$("${CURL[@]}" "${AUTH[@]}" -X PATCH \
            -H 'Content-Type: application/json' -H "If-Match: ${ETAG}" \
            -d "{\"PAMEnabled\":${want}}" -o /dev/null -w '%{http_code}' "${BASE}${CFG}" || true)
  case "$pcode" in
    200|204) ;;
    428) die "PATCH rejected 428 (precondition) — ETag changed under us; re-run" 2;;
    412) die "PATCH rejected 412 (If-Match mismatch) — ETag stale; re-run" 2;;
    401) die "admin auth rejected on PATCH (401)" 3;;
    *)   die "PATCH failed HTTP ${pcode}" 2;;
  esac
  # verify
  NEW=$("${CURL[@]}" "${AUTH[@]}" "${BASE}${CFG}" | python3 -I -c "import sys,json;print(json.load(sys.stdin).get('PAMEnabled'))" 2>/dev/null)
  echo "verified PAMEnabled=${NEW}"
  nlc=$(printf '%s' "$NEW" | tr '[:upper:]' '[:lower:]')
  [ "$nlc" = "$want" ] || die "verify failed: PAMEnabled is '${NEW}', expected '${want}'" 2
fi

# optional: confirm a normal (IPMI/web) user can now authenticate to Redfish
if [ -n "$TEST_USER" ]; then
  [ -n "$TEST_PASS" ] || die "--test-user given but no --test-pass / RF_TEST_PASS" 1
  tcode=$("${CURL[@]}" -u "${TEST_USER}:${TEST_PASS}" -o /dev/null -w '%{http_code}' "${BASE}/redfish/v1/Systems/Self" || true)
  echo "test: '${TEST_USER}' Basic-auth to Redfish Systems/Self -> ${tcode}"
  if [ "$want" = "true" ]; then
    [ "$tcode" = "200" ] && echo "  OK: normal user now authenticates to Redfish." \
      || echo "  NOTE: expected 200 after enable; got ${tcode}. Check the user's privilege / PAMOrder."
  fi
fi

cat <<EOF

reminder: enabling PAM lets normal IPMI/web users authenticate to Redfish, but the
bootstrap account you used (${ADMIN_USER}) may still be a factory-default admin. Rotate
its password separately — it is a full-privilege Redfish login not shown in the web UI.
EOF
