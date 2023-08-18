#!/usr/bin/env bash
wget https://github.com/gogrlx/grlx/releases/download/v0.0.4/sprout
chmod +x sprout
sudo systemctl stop grlx-sprout.service
sudo rm /etc/grlx/pki/sprout/tls-rootca.pem
sudo mv sprout /usr/bin/grlx-sprout
sudo systemctl start grlx-sprout.service
