#!/bin/bash

LATEST_VERSION="v1.8.0"

command_exists () {
    type "$1" &> /dev/null;
}

install_jq () {
    if command_exists apt-get; then
        apt-get -y update
        apt -y -q install jq
    fi

    if ! command_exists jq; then
        echo "jq: command not found"
        exit 1;
    fi
}

check_release () {
    LATEST_VERSION=$(curl -sL https://api.github.com/repos/scrtlabs/SecretNetwork/releases/latest | jq -r ".tag_name");
    GOLEVELDB_LINK=`curl -sL https://api.github.com/repos/scrtlabs/SecretNetwork/releases/latest | jq -r ".assets[].browser_download_url" | grep "$1"_goleveldb_amd64.deb`;
    ROCKSDB_LINK=`curl -sL https://api.github.com/repos/scrtlabs/SecretNetwork/releases/latest | jq -r ".assets[].browser_download_url" | grep "$1"_rocksdb_amd64.deb`;
}

read_installation_method () {
    echo "--------------------------------------------------------------------------------------------";
    echo "This installation file is built for Secret Network release $LATEST_VERSION.";
    echo "This will automatically install a fresh secretd or update an old secretd to either rocksdb or golevel, depending on your choice.";
    echo "Note: If you are running a previous version of secretd with systemd service file, stop the service prior to running this installation.";
    echo "";
    echo "This script will install secretd and enable it as a systemd service. ";
    echo "Make sure you have set up the correct configuration file for your chain and you have a valid node key.";
    echo "Instructions for configuring your node can be found in the official documentation at https://docs.scrt.network/docs/nodes/secret-node/";
    echo "";
    echo "<3 from ChainofSecrets.org";
    echo "--------------------------------------------------------------------------------------------";
    echo "";
    read -p "Do you want to install Secret Network $LATEST_VERSION on mainnet or testnet? [mainnet/testnet]: " network_choice
    read -p "Do you want to install with rocksdb or goleveldb? [rocksdb/goleveldb]: " db_choice
}

if ! command_exists jq ; then
    install_jq;
fi

check_release "testnet";
testnet_goleveldb=$GOLEVELDB_LINK;
testnet_rocksdb=$ROCKSDB_LINK;
check_release "mainnet";
mainnet_goleveldb=$GOLEVELDB_LINK;
mainnet_rocksdb=$ROCKSDB_LINK;

read_installation_method;

if [ $network_choice == "testnet" ]; then
    if [ $db_choice == "goleveldb" ]; then
        wget $testnet_goleveldb;
        apt install -y ./secretnetwork_*_testnet_goleveldb_amd64.deb;
    elif [ $db_choice == "rocksdb" ]; then
        wget $testnet_rocksdb;
        apt install -y ./secretnetwork_*_testnet_rocksdb_amd64.deb;
    else
        echo "Invalid choice for db.";
        exit 1;
    fi
elif [ $network_choice == "mainnet" ]; then
    if [ $db_choice == "goleveldb" ]; then
        wget $mainnet_goleveldb;
        apt install -y ./secretnetwork_*_mainnet_goleveldb_amd64.deb;
   
read -p "Please enter the desired network (mainnet or testnet): " network
if [[ $network != "mainnet" && $network != "testnet" ]]; then
  echo "Invalid network entered. Please try again."
  exit 1
fi

read -p "Please enter the desired database backend (goleveldb or rocksdb): " db_backend
if [[ $db_backend != "goleveldb" && $db_backend != "rocksdb" ]]; then
  echo "Invalid database backend entered. Please try again."
  exit 1
fi

echo "Downloading Secret Network release $latest_version for $network network with $db_backend database backend..."

if [[ $db_backend == "goleveldb" ]]; then
  file_name="secretnetwork_${latest_version}_${network}_goleveldb_amd64.deb"
else
  file_name="secretnetwork_${latest_version}_${network}_rocksdb_amd64.deb"
fi

if ! curl -O "https://github.com/scrtlabs/SecretNetwork/releases/download/v$latest_version/$file_name"; then
  echo "Failed to download Secret Network release. Please try again."
  exit 1
fi

echo "Installing Secret Network release..."

if ! sudo apt install "./$file_name"; then
  echo "Failed to install Secret Network release. Please try again."
  exit 1
fi

echo "Secret Network release installed successfully!"
echo "----------------------------------------------------"
echo "Installation completed successfully!"
echo "----------------------------------------------------"
