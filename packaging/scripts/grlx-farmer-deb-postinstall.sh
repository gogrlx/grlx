#!/bin/bash
# Create farmer user if it doesn't exist
if ! id -u farmer >/dev/null 2>&1; then
  useradd -r -s /bin/false -d /var/cache/grlx/farmer farmer
fi
# Set ownership
chown -R farmer:farmer /etc/grlx/pki/farmer /var/cache/grlx/farmer
# Enable and start service
systemctl daemon-reload
systemctl enable grlx-farmer.service 