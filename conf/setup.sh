#!/bin/bash

set -euo pipefail

echo "Linking sysusers config..."

mkdir -p /etc/sysusers.d

if [ ! -f /etc/sysusers.d/whiskr.conf ]; then
    ln -s "/var/whiskr/conf/whiskr.conf" /etc/sysusers.d/whiskr.conf
fi

echo "Creating user..."
systemd-sysusers

echo "Linking unit..."
rm /etc/systemd/system/whiskr.service

systemctl link "/var/whiskr/conf/whiskr.service"

echo "Reloading daemon..."
systemctl daemon-reload
systemctl enable whiskr

echo "Fixing initial permissions..."
chown -R whiskr:whiskr "/var/whiskr"

find "/var/whiskr" -type d -exec chmod 755 {} +
find "/var/whiskr" -type f -exec chmod 644 {} +

chmod +x "/var/whiskr/whiskr"

echo "Setup complete, starting service..."

service whiskr start

echo "Done."
