#!/bin/bash

# 1 = username
# 2 = moniker
# 3 = chainid
# 4 = persistent peers
# 5 = rpc url (to get genesis file from)
# 6 = registration service (our custom registration helper)
# 7 = docker compose file location

export DEBIAN_FRONTEND=noninteractive

sudo /bin/date +%H:%M:%S > /home/"$1"/install.progress.txt

echo "Creating tmp folder for aesm" >> /home/"$1"/install.progress.txt

# Aesm service relies on this folder and having write permissions
# shellcheck disable=SC2174
mkdir -p -m 777 /tmp/aesmd
chmod -R -f 777 /tmp/aesmd || sudo chmod -R -f 777 /tmp/aesmd || true

echo "Installing docker" >> /home/"$1"/install.progress.txt

sudo apt update
sudo apt install apt-transport-https ca-certificates curl software-properties-common -y
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -

sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable"
sudo apt update
sudo apt install docker-ce -y

echo "Adding user $1 to docker group" >> /home/"$1"/install.progress.txt
sudo service docker start
sudo systemctl enable docker
sudo groupadd docker
sudo usermod -aG docker "$1"

echo "Installing docker-compose" >> /home/"$1"/install.progress.txt
# systemctl status docker
sudo curl -L https://github.com/docker/compose/releases/download/1.26.0/docker-compose-"$(uname -s)"-"$(uname -m)" -o /usr/local/bin/docker-compose

sudo chmod +x /usr/local/bin/docker-compose

echo "Creating secret node runner" >> /home/"$1"/install.progress.txt

mkdir -p /usr/local/bin/secret-node

echo "Copying docker compose file from $7" >> /home/"$1"/install.progress.txt
sudo curl -L "$7" -o /usr/local/bin/secret-node/docker-compose.yaml

mainnetstr="mainnet"
if test "${6#*$mainnetstr}" != "$6"
then
  echo "Running with mainnet config" >> /home/"$1"/install.progress.txt
else
  # leaving this here as a placeholder for future versions where we might have to change stuff for testnet vs. mainnet
  echo "Running with testnet config" >> /home/"$1"/install.progress.txt
fi


# replace the tmp paths with home directory ones
sudo sed -i 's/\/tmp\/.secretd:/\/home\/'$1'\/.secretd:/g' /usr/local/bin/secret-node/docker-compose.yaml
sudo sed -i 's/\/tmp\/.secretcli:/\/home\/'$1'\/.secretcli:/g' /usr/local/bin/secret-node/docker-compose.yaml
sudo sed -i 's/\/tmp\/.sgx_secrets:/\/home\/'$1'\/.sgx_secrets:/g' /usr/local/bin/secret-node/docker-compose.yaml

# Open RPC port to the public
perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml

# Open P2P port to the outside
perl -i -pe 's/laddr = .+?26656"/laddr = "tcp:\/\/0.0.0.0:26656"/' ~/.secretd/config/config.toml

echo "Setting Secret Node environment variables and aliases" >> /home/"$1"/install.progress.txt

export CHAINID=$2
export MONIKER=$3
export PERSISTENT_PEERS=$4
export RPC_URL=$5
export REGISTRATION_SERVICE=$6

# set Aliases and environment variables
{
  echo 'alias secretcli="docker exec -it secret-node_node_1 secretcli"'
  echo 'alias secretd="docker exec -it secret-node_node_1 secretd"'
  echo 'alias show-node-id="docker exec -it secret-node_node_1 secretd tendermint show-node-id"'
  echo 'alias show-validator="docker exec -it secret-node_node_1 secretd tendermint show-validator"'
  echo 'alias stop-secret-node="docker-compose -f /usr/local/bin/secret-node/docker-compose.yaml down"'
  echo 'alias start-secret-node="docker-compose -f /usr/local/bin/secret-node/docker-compose.yaml up -d"'
  echo "export CHAINID=$2"
  echo "export MONIKER=$3"
  echo "export PERSISTENT_PEERS=$4"
  echo "export RPC_URL=$5"
  echo "export REGISTRATION_SERVICE=$6"
} >> /home/"$1"/.bashrc

# Log these for debugging purposes
{
  echo "CHAINID=$2"
  echo "MONIKER=$3"
  echo "PRSISTENT_PEERS=$4"
  echo "RPC_URL=$5"
  echo "REGISTRATION_SERVICE=$6"
} >> /home/"$1"/install.progress.txt

################################################################
# Configure to auto start at boot					    #
################################################################
file=/etc/init.d/secret-node
if [ ! -e "$file" ]
then
  {
    echo '#!/bin/sh'
    printf '\n'
    # shellcheck disable=SC2016
    printf '### BEGIN INIT INFO
# Provides:       secret-node
# Required-Start:    $all
# Required-Stop:     $local_fs $network $syslog $named $docker
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: starts secret node
# Description:       starts secret node running in docker
### END INIT INFO\n\n'
    printf 'mkdir -p -m 777 /tmp/aesmd\n'
    printf 'chmod -R -f 777 /tmp/aesmd || sudo chmod -R -f 777 /tmp/aesmd || true\n'
    printf '\n'
    echo "export CHAINID=$2"
    echo "export MONIKER=$3"
    echo "export PRSISTENT_PEERS=$4"
    echo "export RPC_URL=$5"
    echo "export REGISTRATION_SERVICE=$6"
    printf 'docker-compose -f /usr/local/bin/secret-node/docker-compose.yaml up -d\n'
  } | sudo tee /etc/init.d/secret-node

	sudo chmod +x /etc/init.d/secret-node
	sudo update-rc.d secret-node defaults
fi

docker-compose -f /usr/local/bin/secret-node/docker-compose.yaml up -d

secretcli completion > /root/secretcli_completion
secretd completion > /root/secretd_completion

docker cp secret-node_node_1:/root/secretcli_completion /home/"$1"/secretcli_completion
docker cp secret-node_node_1:/root/secretd_completion /home/"$1"/secretd_completion

echo 'source /home/'$1'/secretd_completion' >> /home/"$1"/.bashrc
echo 'source /home/'$1'/secretcli_completion' >> /home/"$1"/.bashrc

echo "Secret Node has been setup successfully and is running..." >> /home/"$1"/install.progress.txt
