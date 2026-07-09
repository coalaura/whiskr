#!/bin/bash

set -euo pipefail

echo "Stopping service..."
systemctl stop "whiskr_proxy" 2>/dev/null || true

echo "Disabling service..."
systemctl disable "whiskr_proxy" 2>/dev/null || true

echo "Removing unit file..."
rm -f "/etc/systemd/system/whiskr_proxy.service"

echo "Removing sysusers config..."
rm -f "/etc/sysusers.d/whiskr_proxy.conf"

if [ -f "/etc/logrotate.d/whiskr_proxy" ]; then
    echo "Removing logrotate config..."
    rm -f "/etc/logrotate.d/whiskr_proxy"
fi

echo "Reloading daemon..."
systemctl daemon-reload
systemctl reset-failed "whiskr_proxy" 2>/dev/null || true

echo "Removing user and group..."
if id "whiskr_proxy" &>/dev/null; then
    userdel "whiskr_proxy" 2>/dev/null || true
fi

if getent group "whiskr_proxy" &>/dev/null; then
    groupdel "whiskr_proxy" 2>/dev/null || true
fi

echo "Uninstall complete."
