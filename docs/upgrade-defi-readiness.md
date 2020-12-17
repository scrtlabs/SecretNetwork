# :warning: WIP :warning:
# DeFi Readiness Network Upgrade (Codename: Ferenginar)

```bash
# backup old version
sudo cp "$(which secretd)" secretd-v1.0.0

# download new version
wget -O secretd-v1.0.4 https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.4/secretd

# check integrity of new version
echo "91b2db0af37fab5bfc8d2eee4ca9b2c075ca97e0c487a4ed12892df81176bd50 secretd-v1.0.4" | sha256sum --check

# stop the node
sudo systemctl stop secret-node

# overwrite the old version with the new version while preserving the file permissions
sudo sh -c "cat secretd-v1.0.4 > '$(which secretd)'"

# start the node
sudo systemctl start secret-node
```
