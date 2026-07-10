#!/bin/sh
# deb postinstall: nudge app_mgr to pick up the new application.
set +e

echo ""
echo "noports-srlinux installed."
echo ""
echo "Next steps (SR Linux CLI):"
echo "  enter candidate"
echo "  set / noports device-atsign @mydevice"
echo "  set / noports manager-atsigns [ @manager ]"
echo "  set / noports device-name srl-router-1"
echo "  set / noports admin-state enable"
echo "  commit now"
echo ""
echo "Then enroll this device via APKAM (cuts keys on the router):"
echo "  bash"
echo "  sudo /opt/noports/onboard-noports.sh <otp/spp passcode>"
echo ""

# Reload app_mgr automatically when running on a real SR Linux system
# (not in a package build container).
if [ -f /etc/profile.d/sr_app_env.sh ]; then
    # shellcheck disable=SC1091
    . /etc/profile.d/sr_app_env.sh
fi
if command -v sr_cli >/dev/null 2>&1; then
    echo "Reloading app_mgr"
    sr_cli -e "/ tools system app-management application app_mgr reload" >/dev/null 2>&1 || true
fi
