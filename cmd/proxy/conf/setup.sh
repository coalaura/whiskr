#!/bin/bash

set -euo pipefail

# --- HARDWARE PERMISSIONS ---
# If this service requires hardware access, you likely need a udev rule
# to assign ownership to the 'whiskr_proxy' user.
# Example: /etc/udev/rules.d/99-whiskr_proxy.rules
# SUBSYSTEM=="usb", ATTRS{idVendor}=="XXXX", OWNER="whiskr_proxy"
# ----------------------------

echo "Linking sysusers config..."

mkdir -p /etc/sysusers.d

if [ -f /etc/sysusers.d/whiskr_proxy.conf ]; then
    rm /etc/sysusers.d/whiskr_proxy.conf
fi

ln -s "/var/proxy.example.com/conf/whiskr_proxy.conf" /etc/sysusers.d/whiskr_proxy.conf

echo "Creating user..."

systemd-sysusers

echo "Linking unit..."

if [ -f /etc/systemd/system/whiskr_proxy.service ]; then
    rm /etc/systemd/system/whiskr_proxy.service
fi

systemctl link "/var/proxy.example.com/conf/whiskr_proxy.service"

if command -v logrotate >/dev/null 2>&1; then
    echo "Linking logrotate config..."

    if [ -f /etc/logrotate.d/whiskr_proxy ]; then
        rm /etc/logrotate.d/whiskr_proxy
    fi

    ln -s "/var/proxy.example.com/conf/whiskr_proxy_logs.conf" /etc/logrotate.d/whiskr_proxy
else
    echo "Logrotate not found, skipping..."
fi

echo "Reloading daemon..."

systemctl daemon-reload
systemctl enable whiskr_proxy

echo "Fixing initial permissions..."

mkdir -p "/var/proxy.example.com/logs"

chown -R whiskr_proxy:whiskr_proxy "/var/proxy.example.com"

find "/var/proxy.example.com" -type d -exec chmod 755 {} +
find "/var/proxy.example.com" -type f -exec chmod 644 {} +

chmod +x "/var/proxy.example.com/whiskr_proxy"

echo "Setup complete, starting service..."

service whiskr_proxy restart

echo "Done."
