### Download Release 0.0.1

```shell
wget https://github.com/enigmampc/EnigmaBlockchain/releases/download/v0.0.2/enigmachain_0.0.2_amd64.deb
```

### Remove old installations

```shell
sudo dpkg -r enigmachain
sudo rm -rf ~/.enigmad ~/.enigmacli
sudo rm -rf ~/.engd ~/.engcli
sudo rm -rf "$(which enigmad)"
sudo rm -rf "$(which enigmacli)"
sudo rm -rf "$(which engcli)"
sudo rm -rf "$(which engd)"
```

### Configure LVM

- Attach storage to instance
- Run the following

```shell
# Create volumes and groups
sudo pvcreate /dev/xvdf
sudo vgcreate chainstate /dev/xvdf
sudo lvcreate --name data --size 19GB chainstate

# Format to ext4
sudo mkfs.ext4 /dev/chainstate/data

# Create the `data` path and mount
sudo mkdir -p .enigmad/data
sudo mount /dev/chainstate/data .enigmad/data

# Make mount persistant
sudo echo "/dev/chainstate/data	/home/ubuntu ext4 defaults		0 0" >> /etc/fstab

# Make the default user able to r/w
sudo chown -R ubuntu .enigmad/
```

### Install the `.deb` file

```shell
sudo dpkg -i enigmachain_0.0.2_amd64.deb
```

### Config local node

```shell
enigmacli config chain-id "enigma-testnet"
enigmacli config output json
enigmacli config indent true
enigmacli config trust-node true # true if you trust the full-node you are connecting to, false otherwise
```
