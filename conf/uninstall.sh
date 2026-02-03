#!/bin/bash

set -euo pipefail

echo "Stopping service..."
systemctl stop "whiskr" 2>/dev/null || true

echo "Disabling service..."
systemctl disable "whiskr" 2>/dev/null || true

echo "Removing unit file..."
rm -f "/etc/systemd/system/whiskr.service"

echo "Removing sysusers config..."
rm -f "/etc/sysusers.d/whiskr.conf"

if [ -f "/etc/logrotate.d/whiskr" ]; then
    echo "Removing logrotate config..."
    rm -f "/etc/logrotate.d/whiskr"
fi

echo "Reloading daemon..."
systemctl daemon-reload
systemctl reset-failed "whiskr" 2>/dev/null || true

echo "Removing user and group..."
if id "whiskr" &>/dev/null; then
    userdel "whiskr" 2>/dev/null || true
fi

if getent group "whiskr" &>/dev/null; then
    groupdel "whiskr" 2>/dev/null || true
fi

echo "Uninstall complete."
