#!/bin/bash

set -euo pipefail

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

echo "Reloading daemon..."
systemctl daemon-reload
systemctl enable whiskr

echo "Fixing initial permissions..."
chown -R whiskr:whiskr "/var/wskr.sh"

find "/var/wskr.sh" -type d -exec chmod 755 {} +
find "/var/wskr.sh" -type f -exec chmod 644 {} +

chmod +x "/var/wskr.sh/whiskr"

echo "Setup complete, starting service..."

service whiskr start

echo "Done."
