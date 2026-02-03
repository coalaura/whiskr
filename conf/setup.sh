#!/bin/bash

set -euo pipefail

# --- HARDWARE PERMISSIONS ---
# If this service requires hardware access, you likely need a udev rule
# to assign ownership to the 'whiskr' user.
# Example: /etc/udev/rules.d/99-whiskr.rules
# SUBSYSTEM=="usb", ATTRS{idVendor}=="XXXX", OWNER="whiskr"
# ----------------------------

echo "Linking sysusers config..."

mkdir -p /etc/sysusers.d

if [ -f /etc/sysusers.d/whiskr.conf ]; then
    rm /etc/sysusers.d/whiskr.conf
fi

ln -s "/var/wskr.sh/conf/whiskr.conf" /etc/sysusers.d/whiskr.conf

echo "Creating user..."

systemd-sysusers

echo "Linking unit..."

if [ -f /etc/systemd/system/whiskr.service ]; then
    rm /etc/systemd/system/whiskr.service
fi

systemctl link "/var/wskr.sh/conf/whiskr.service"

if command -v logrotate >/dev/null 2>&1; then
    echo "Linking logrotate config..."

    if [ -f /etc/logrotate.d/whiskr ]; then
        rm /etc/logrotate.d/whiskr
    fi

    ln -s "/var/wskr.sh/conf/whiskr_logs.conf" /etc/logrotate.d/whiskr
else
    echo "Logrotate not found, skipping..."
fi

echo "Reloading daemon..."

systemctl daemon-reload
systemctl enable whiskr

echo "Fixing initial permissions..."

mkdir -p "/var/wskr.sh/logs"

chown -R whiskr:whiskr "/var/wskr.sh"

find "/var/wskr.sh" -type d -exec chmod 755 {} +
find "/var/wskr.sh" -type f -exec chmod 644 {} +

chmod +x "/var/wskr.sh/whiskr"

echo "Setup complete, starting service..."

service whiskr restart

echo "Done."
