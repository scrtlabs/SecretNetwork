# DeFi Readiness Network Upgrade (Codename: Ferenginar)

```bash
# backup old version
sudo cp "$(which secretd)" secretd-v1.0.0

# download new version
wget -O secretd-v1.0.4 https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.4/secretd

# check integrity of new version
echo "4ba817f2f5dba092359ec26e5aaedec7df41c370469a8879ef24898fbd38c8e7 secretd-v1.0.4" | sha256sum --check

# stop the node
sudo systemctl stop secret-node

# overwrite the old version with the new version while preserving the file permissions
sudo sh -c "cat secretd-v1.0.4 > '$(which secretd)'"

# start the node
sudo systemctl start secret-node
```
