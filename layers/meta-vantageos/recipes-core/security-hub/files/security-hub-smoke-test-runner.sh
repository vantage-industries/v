#!/bin/sh
# Gates security-hub/backend's own deploy/smoke-test.sh so it runs unattended exactly once,
# and only after two things a fresh appliance cannot supply itself:
#   1. the owner account exists (backend refuses default/hardcoded credentials -- owner is
#      created interactively via /setup/bootstrap, never by this image)
#   2. the operator has dropped the owner password into ENV_FILE after doing that bootstrap
#
# Triggered on a timer (security-hub-smoke-test.timer) rather than gating boot, since both
# conditions above depend on the operator, not the OS.
set -eu

DONE_MARKER=/var/lib/securityhub/smoke-test.done
ENV_FILE=/etc/securityhub/smoke-test.env
LOG=/var/log/securityhub-smoke-test.log

[ -f "$DONE_MARKER" ] && exit 0
[ -f "$ENV_FILE" ] || exit 0

# shellcheck disable=SC1090
. "$ENV_FILE"
[ -n "${PASSWORD:-}" ] || exit 0

STATUS="$(wget -q -O - http://127.0.0.1:8080/api/v1/setup/status 2>/dev/null || true)"
case "$STATUS" in
    *'"setup_required":true'*) exit 0 ;;   # not bootstrapped yet -- try again next tick
    *'"setup_required":false'*) ;;          # bootstrapped -- proceed
    *) exit 0 ;;                            # API not reachable yet -- try again next tick
esac

# smoke-test.sh enrols a fixed test MAC and is not safe to re-run blind (a second pass hits
# "duplicate mac"). Mark done before running so this fires exactly once, matching the
# "once, at first opportunity after bootstrap" contract -- not "retry until success".
install -d -m 0750 -o securityhub -g securityhub "$(dirname "$DONE_MARKER")"
touch "$DONE_MARKER"

export PASSWORD
/usr/bin/security-hub-smoke-test >"$LOG" 2>&1 || true
