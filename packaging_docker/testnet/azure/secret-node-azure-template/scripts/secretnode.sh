#!/bin/bash

# 1 = username
# 2 = moniker
# 3 = chainid
# 4 = persistent peers
# 5 = rpc url (to get genesis file from)
# 6 = registration service (our custom registration helper)

export DEBIAN_FRONTEND=noninteractive

sudo /bin/date +%H:%M:%S > /home/$1/install.progress.txt

echo "Creating tmp folder for aesm" >> /home/$1/install.progress.txt

# Aesm service relies on this folder and having write permissions
# shellcheck disable=SC2174
mkdir -p -m 777 /tmp/aesmd
chmod -R -f 777 /tmp/aesmd || sudo chmod -R -f 777 /tmp/aesmd || true

echo "Installing docker" >> /home/$1/install.progress.txt

sudo apt update
sudo apt install apt-transport-https ca-certificates curl software-properties-common -y
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -

sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable"
sudo apt update
sudo apt install docker-ce -y

echo "Adding user $1 to docker group" >> /home/$1/install.progress.txt
sudo service docker start
sudo systemctl enable docker
sudo groupadd docker
sudo usermod -aG docker $1

echo "Installing docker-compose" >> /home/$1/install.progress.txt
# systemctl status docker
sudo curl -L https://github.com/docker/compose/releases/download/1.26.0/docker-compose-"$(uname -s)"-"$(uname -m)" -o /usr/local/bin/docker-compose

sudo chmod +x /usr/local/bin/docker-compose

echo "Creating secret node runner" >> /home/$1/install.progress.txt

mkdir -p /usr/local/bin/secret-node

sudo curl -L https://raw.githubusercontent.com/enigmampc/SecretNetwork/develop/packaging_docker/testnet/azure/secret-node-azure-template/scripts/docker-compose.yaml -o /usr/local/bin/secret-node/docker-compose.yaml

# replace the tmp paths with home directory ones
sudo sed -i 's/\/tmp\/.secretd:/\/home\/'$1'\/.secretd:/g' /usr/local/bin/secret-node/docker-compose.yaml
sudo sed -i 's/\/tmp\/.secretcli:/\/home\/'$1'\/.secretcli:/g' /usr/local/bin/secret-node/docker-compose.yaml
sudo sed -i 's/\/tmp\/.sgx_secrets:/\/home\/'$1'\/.sgx_secrets:/g' /usr/local/bin/secret-node/docker-compose.yaml


# Open RPC port to the public
perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml

# Open P2P port to the outside
perl -i -pe 's/laddr = .+?26656"/laddr = "tcp:\/\/0.0.0.0:26656"/' ~/.secretd/config/config.toml

echo "Setting Secret Node environment variables" >> /home/$1/install.progress.txt

echo 'alias secretcli="docker exec -it secret-node_node_1 secretcli"' >> /home/"$1"/.bashrc
echo 'alias secretd="docker exec -it secret-node_node_1 secretd"' >> /home/"$1"/.bashrc
echo 'alias show-node-id="docker exec -it bootstrap secretd tendermint show-node-id"' >> /home/"$1"/.bashrc
echo 'alias show-validator="docker exec -it bootstrap secretd tendermint show-node-id"' >> /home/"$1"/.bashrc

echo 'alias stop-secret-node="docker-compose -f /usr/local/bin/secret-node/docker-compose.yaml down"' >> /home/"$1"/.bashrc
echo 'alias start-secret-node="docker-compose -f /usr/local/bin/secret-node/docker-compose.yaml up -d"' >> /home/"$1"/.bashrc

echo "export CHAINID=$2" >> /home/"$1"/.bashrc
echo "export MONIKER=$3" >> /home/"$1"/.bashrc
echo "export PERSISTENT_PEERS=$4" >> /home/"$1"/.bashrc
echo "export RPC_URL=$5" >> /home/"$1"/.bashrc
echo "export REGISTRATION_SERVICE=$6" >> /home/"$1"/.bashrc
# echo "export GENESIS_PATH=$5" >> /home/$1/.bashrc

export CHAINID=$2
export MONIKER=$3
export PERSISTENT_PEERS=$4
export RPC_URL=$5
export REGISTRATION_SERVICE=$6

echo "CHAINID=$2" >> /home/"$1"/install.progress.txt
echo "MONIKER=$3" >> /home/"$1"/install.progress.txt
echo "PRSISTENT_PEERS=$4" >> /home/"$1"/install.progress.txt
echo "export RPC_URL=$5" >> /home/"$1"/install.progress.txt
echo "export REGISTRATION_SERVICE=$6" >> /home/"$1"/install.progress.txt

################################################################
# Configure to auto start at boot					    #
################################################################
file=/etc/init.d/secret-node
if [ ! -e "$file" ]
then
	printf '%s\n%s\n' '#!/bin/sh' 'docker-compose -f /usr/local/bin/secret-node/docker-compose.yaml up -d' | sudo tee /etc/init.d/secret-node
	sudo chmod +x /etc/init.d/secret-node
	sudo update-rc.d secret-node defaults
fi

docker-compose -f /usr/local/bin/secret-node/docker-compose.yaml up -d
echo "Secret Node has been setup successfully and is running..." >> /home/$1/install.progress.txt
