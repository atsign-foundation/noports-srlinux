#!/usr/bin/env bash
# Launcher for NoPorts sshnpd under SR Linux app_mgr.
#
# Installed to /opt/sshnpd/run-sshnpd.sh and invoked by app_mgr
# (see /etc/opt/srlinux/appmgr/sshnpd.yml).
#
# All deployment-specific settings come from /etc/opt/sshnpd/sshnpd.env.
set -euo pipefail

ENV_FILE=/etc/opt/sshnpd/sshnpd.env
if [ ! -f "$ENV_FILE" ]; then
    echo "sshnpd: $ENV_FILE not found; copy sshnpd.env.example and edit it" >&2
    exit 1
fi
# shellcheck disable=SC1090
source "$ENV_FILE"

: "${DEVICE_ATSIGN:?set DEVICE_ATSIGN in $ENV_FILE}"
: "${MANAGER_ATSIGN:?set MANAGER_ATSIGN in $ENV_FILE}"
: "${DEVICE_NAME:?set DEVICE_NAME in $ENV_FILE}"
KEY_FILE="${KEY_FILE:-/etc/opt/sshnpd/keys/${DEVICE_ATSIGN}_key.atKeys}"
SSHNPD_BIN="${SSHNPD_BIN:-/usr/local/bin/sshnpd}"
# Behind restrictive egress ACLs, set ROOT_SERVER="proxy:<host>:443" in the
# env file to send all atProtocol traffic to a reverse proxy on one port.
ROOT_SERVER="${ROOT_SERVER:-root.atsign.org}"
EXTRA_ARGS="${EXTRA_ARGS:-}"

if [ ! -f "$KEY_FILE" ]; then
    echo "sshnpd: atKeys file $KEY_FILE not found;" \
         "run /opt/sshnpd/onboard-sshnpd.sh to enroll this device" >&2
    exit 1
fi

# sshnpd keeps its local storage under $HOME; give it a persistent,
# writable location rather than whatever HOME app_mgr inherited.
export HOME=/var/opt/sshnpd
mkdir -p "$HOME"

# The management VRF (and the local sshd) live in the srbase-mgmt network
# namespace; the default namespace has no route to the internet.
# shellcheck disable=SC2086
exec ip netns exec srbase-mgmt "$SSHNPD_BIN" \
    --key-file "$KEY_FILE" \
    --atsign "$DEVICE_ATSIGN" \
    --managers "$MANAGER_ATSIGN" \
    --device "$DEVICE_NAME" \
    --root-server "$ROOT_SERVER" \
    $EXTRA_ARGS
