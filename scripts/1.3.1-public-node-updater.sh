#!/bin/bash
# ChainofSecrets.org - Public Release.
# Set "mainnet" or "testnet"

NETWORK="mainnet";

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

install_secretd () {
        if command_exists wget; then
                wget "$GOLEVELDB_LINK"
  	        sudo apt install -y ./secretnetwork_"$LATEST_VERSION"_"$NETWORK"_goleveldb_amd64.deb    
        fi

        if ! command_exists secretd; then
                echo "secretd: command not found"
                exit 1;
        fi
}

install_sgx () {

	wget "https://raw.githubusercontent.com/SecretFoundation/docs/main/docs/node-guides/sgx"
	sudo bash sgx
}

check_release () {

        LATEST_VERSION=$(curl -sL https://api.github.com/repos/scrtlabs/secretnetwork/releases/latest | jq -r ".tag_name" | sed 's/v//');
        GOLEVELDB_LINK=`curl -sL https://api.github.com/repos/scrtlabs/secretnetwork/releases/latest | jq -r ".assets[].browser_download_url" | grep "$NETWORK"_goleveldb_amd64.deb`;
        ROCKSDB_LINK=`curl -sL https://api.github.com/repos/scrtlabs/secretnetwork/releases/latest | jq -r ".assets[].browser_download_url" | grep "$NETWORK"_rocksdb_amd64.deb`;

}

read_installation_method () {
	echo "--------------------------------------------------------------------------------------------";
	echo "This installation file is built for any version of secretd.";
	echo "Note: Currently does not support SGX check then install, therefor will just install SGX again.";
	echo "";
	echo "This will automatically install a fresh secretd or update an old secretd to either rocksdb or golevel";
	echo "depending on current running configuration.";
	echo "";
	echo "<3 from ChainofSecrets.org";
	echo "--------------------------------------------------------------------------------------------";
	echo "";
	read -p "Install secretd [Y/N]? " choice
	case "$choice" in
		y|Y )
			INSTALL="true";
		;;

		n|N )
			exit 0;
		;;

		* )
			echo "Please, enter Y or N to cancel";
		;;
	esac
}

if ! command_exists jq ; then
        install_jq;
fi

check_release;
read_installation_method;
install_sgx;

if ! command_exists secretd ; then
        install_secretd;
fi

        
if [ $(secretd version) = "$LATEST_VERSION" ]; then 
  echo 'Current Version '$LATEST_VERSION' - Updating not needed! Happy Nodeling - ChainofSecrets.org'
else
 if [ $(awk -F \" '/^db_backend =/{print $2}' ~/.secretd/config/config.toml) = 'goleveldb' ]; then
	echo "This is a golevelDB install"
  	sudo systemctl stop secret-node
  	wget "$GOLEVELDB_LINK"
  	sudo apt install -y ./secretnetwork_"$LATEST_VERSION"_"$NETWORK"_goleveldb_amd64.deb
  
 else

  	echo "This is a Rocksdb install"
  	sudo systemctl stop secret-node
  	wget "$ROCKSDB_LINK"
  	sudo apt install -y ./secretnetwork_"$LATEST_VERSION"_"$NETWORK"_rocksdb_amd64.deb
 fi


# .Restart the node & modify the service

perl -i -pe 's{^(ExecStart=).*}{ExecStart=\/usr\/local\/bin\/secretd start}' /etc/systemd/system/secret-node.service

systemctl daemon-reload

sudo systemctl start secret-node

fi
