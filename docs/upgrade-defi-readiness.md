# DeFi Readiness Network Upgrade (Codename: Ferenginar)

```bash
# backup old version
sudo cp "$(which secretd)" secretd-v1.0.0

# download new version
wget -O secretd-v1.0.4 https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.4/secretd

# stop the node
sudo systemctl stop secret-node

# overwrite the old version with the new version while preserving the file permissions
sudo sh -c "cat secretd-v1.0.4 > '$(which secretd)'"

# start the node
sudo systemctl start secret-node
```
