ENABLE_FAUCET=${1:-"true"}
KEYRING=${KEYRING:-"test"}
custom_script_path=${POST_INIT_SCRIPT:-"/root/post_init.sh"}
LOG_LEVEL=trace

function InitBootstrap() {

      if [ ${ENABLE_FAUCET} = "true" ]; then
            _pid_=$(ps -ef | grep node.*faucet.* | grep -v grep | awk '{print $2}')
            if [ ! -z "${_pid_}" ]; then
                  echo "Faucet app is running with PID:${_pid_}. Stopping..."
                  kill -HUP ${_pid_} && echo "Successfully stopped PID:" {$_pid_}
            fi
      fi

      v_mnemonic="push certain add next grape invite tobacco bubble text romance again lava crater pill genius vital fresh guard great patch knee series era tonight"
      a_mnemonic="grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar"
      b_mnemonic="jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"
      c_mnemonic="chair love bleak wonder skirt permit say assist aunt credit roast size obtain minute throw sand usual age smart exact enough room shadow charge"
      d_mnemonic="word twist toast cloth movie predict advance crumble escape whale sail such angry muffin balcony keen move employ cook valve hurt glimpse breeze brick"
      x_mnemonic="black foot thrive monkey tenant fashion blouse general adult orient grass enact eight tiger color castle rebuild puzzle much gap connect slice print gossip"
      z_mnemonic="obscure arrest leader echo truth puzzle police evolve robust remain vibrant name firm bulk filter mandate library mention walk can increase absurd aisle token"

      echo $v_mnemonic | secretd keys add validator --recover
      echo $a_mnemonic | secretd keys add a --recover
      echo $b_mnemonic | secretd keys add b --recover
      echo $c_mnemonic | secretd keys add c --recover
      echo $d_mnemonic | secretd keys add d --recover
      echo $z_mnemonic | secretd keys add z --recover

      secretd keys list --output json | jq

      ico=1000000000000000000

      secretd genesis add-genesis-account validator ${ico}uscrt
      secretd genesis add-genesis-account a ${ico}uscrt
      secretd genesis add-genesis-account b ${ico}uscrt
      secretd genesis add-genesis-account c ${ico}uscrt
      secretd genesis add-genesis-account d ${ico}uscrt
      secretd genesis add-genesis-account z ${ico}uscrt

      secretd genesis gentx validator ${ico}uscrt --chain-id "$chain_id"

      secretd genesis collect-gentxs
      secretd genesis validate-genesis

      secretd init-bootstrap
      secretd genesis validate-genesis

      if [ "${ENABLE_FAUCET}" = "true" ]; then
            # Setup faucet
            setsid /usr/bin/node ./faucet/faucet_server.js &
      fi

      # secretd keys list | jq
      # echo $x_mnemonic | secretd keys add userx --recover
      # x_address=$(secretd keys show -a userx)
      # # Now that we have genesis with some genesis accounts - load up the new wallet userx
      # curl http://${FAUCET_URL}/faucet?address=${x_address}
}

function InitNode() {
      echo "Initializing chain: $chain_id with node moniker: $MONIKER"
      # This node is not ready yet, temporarily point it to a bootstrap node
      secretd config set client node tcp://${RPC_URL}

      echo "Give a bootstrap node time to start..."
      sleep 5s

      # Download genesis.json from the bootstrap node
      curl http://${RPC_URL}/genesis | jq '.result.genesis' >${SCRT_HOME}/config/genesis.json

      if [ ! -e ${SCRT_HOME}/config/genesis.json ]; then
            echo "Genesis file failed to download"
            exit 1
      fi
      # verify genesis.json checksum
      cat ${SCRT_HOME}/config/genesis.json | sha256sum
      cat ${SCRT_HOME}/config/genesis.json | jq

      secretd init-enclave
      if [ $? -ne 0 ]; then
            echo "Error: failed to initialize enclave"
            exit 1
      fi

      if [ ! -e $SCRT_SGX_STORAGE/attestation_cert.der ]; then
            echo "Error: failed to generate attestation certificate"
            exit 1
      fi

      # Verify enclave initialization
      ls -lh $SCRT_SGX_STORAGE/attestation_cert.der

      # Extract public key from certificate
      PUBLIC_KEY=$(secretd parse $SCRT_SGX_STORAGE/attestation_cert.der 2>/dev/null | cut -c 3-)
      echo "Public key: ${PUBLIC_KEY}"

      a_mnemonic="grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar"
      mnemonic_userx="black foot thrive monkey tenant fashion blouse general adult orient grass enact eight tiger color castle rebuild puzzle much gap connect slice print gossip"
      echo $mnemonic_userx | secretd keys add userx --recover
      echo $a_mnemonic | secretd keys add a --recover
      secretd keys list --output json | jq

      x_address=$(secretd keys show -a userx)
      if [ -z $x_address ]; then
            echo "Error: cannot find key userx"
            exit 1
      fi

      echo "Address userx=${x_address}"
      txhash=$(secretd tx register auth ${SCRT_SGX_STORAGE}/attestation_cert.der -y --from a --fees 3000uscrt | jq '.txhash' | sed 's/"//g')
      sleep 5s
      secretd q tx --type hash ${txhash}

      # Pull and check this node's encrypted seed from the network
      SEED=$(secretd query register seed $PUBLIC_KEY | cut -c 3-)
      if [ -z ${SEED} ]; then
            echo "Error: failed to pull this node's seed from the network"
            exit 1
      fi
      echo "This node's seed: ${SEED}"

      # Set additiona network parameters necessary before the node start

      secretd query register secret-network-params
      ls -lh ./io-master-key.txt ./node-master-key.txt

      mkdir -p ${SCRT_HOME}/.node
      secretd configure-secret node-master-key.txt $SEED

      # Update your SGX memory enclave cache
      sed -i.bak -e "s/^contract-memory-enclave-cache-size *=.*/contract-memory-enclave-cache-size = \"15\"/" ${SCRT_HOME}/config/app.toml

      # Set minimum gas price
      perl -i -pe 's/^minimum-gas-prices = .+?$/minimum-gas-prices = "0.0125uscrt"/' ${SCRT_HOME}/config/app.toml

      # Get this node's id
      secretd tendermint show-node-id

      # Get ready to run our own node:
      # Open RPC port to all interfaces
      perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ${SCRT_HOME}/config/config.toml
      # Open P2P port to all interfaces
      perl -i -pe 's/laddr = .+?26656"/laddr = "tcp:\/\/0.0.0.0:26656"/' ${SCRT_HOME}/config/config.toml

      secretd config set client node tcp://0.0.0.0:26657

      secretd init-bootstrap ./node-master-key.txt ./io-master-key.txt 

      return 0

      RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --log_level ${LOG_LEVEL} &

      sleep 10s

      # Now that we have genesis with some genesis accounts - load up the new wallet userx
      curl http://${FAUCET_URL}/faucet?address=${x_address}
      # cp /opt/secret/.sgx_secrets/attestation_cert.der ./
      sleep 5s

      secretd q bank balances ${x_address} | jq

      echo "<<<<<=====================================>>>>>"
      echo "Setting this node up as a validator"
      staking_amount=1000000uscrt

      echo "Staking amount: $staking_amount"

      secretd tx staking create-validator \
            --amount=$staking_amount \
            --pubkey=$(secretd tendermint show-validator) \
            --from=userx \
            --moniker=$(hostname) \
            --commission-rate="0.10" \
            --commission-max-rate="0.20" \
            --commission-max-change-rate="0.01" \
            --min-self-delegation="1"

}

if [ -z ${BOOTSTRAP+x} ]; then
      InitNode
#      RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:36657 --log_level ${LOG_LEVEL}
else
      InitBootstrap
      RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap --log_level ${LOG_LEVEL}
fi
