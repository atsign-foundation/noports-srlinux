#!/bin/sh
# deb postinstall: nudge app_mgr to pick up the new application.
set -e

echo ""
echo "noports-srlinux installed."
echo ""
echo "Next steps:"
echo "  1. cp /etc/opt/sshnpd/sshnpd.env.example /etc/opt/sshnpd/sshnpd.env && edit it"
echo "  2. Copy your device atKeys file into /etc/opt/sshnpd/keys/"
echo "  3. Reload app_mgr so SR Linux discovers the app:"
echo "       sr_cli 'tools system app-management application app_mgr reload'"
echo ""

# Try the reload automatically when sr_cli is available (i.e. we are on a
# real SR Linux system, not a build container).
if command -v sr_cli >/dev/null 2>&1; then
    sr_cli 'tools system app-management application app_mgr reload' || true
fi
