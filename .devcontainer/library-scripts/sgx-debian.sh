#! /bin/bash

# TODO: This is currently unused - it would probably be good to clean up the main dockerfile a bit and move all the
# TODO: sgx stuff here

# SGX Binaries
SGX_VERSION=2.12.100.3
OS_REVESION=focal1
LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/

UPDATE_RC=${1:-"true"}

updaterc() {
    if [ "${UPDATE_RC}" = "true" ]; then
        echo "Updating /etc/bash.bashrc and /etc/zsh/zshrc..."
        if [[ "$(cat /etc/bash.bashrc)" != *"$1"* ]]; then
            echo -e "$1" >> /etc/bash.bashrc
        fi
        if [ -f "/etc/zsh/zshrc" ] && [[ "$(cat /etc/zsh/zshrc)" != *"$1"* ]]; then
            echo -e "$1" >> /etc/zsh/zshrc
        fi
    fi
}

sudo apt-get update && sudo apt upgrade -y
sudo apt-get install make build-essential gcc git jq chrony -y

UBUNTUVERSION=$(lsb_release -r -s | cut -d '.' -f 1)
PSW_PACKAGES='libsgx-enclave-common libsgx-urts sgx-aesm-service libsgx-uae-service autoconf libtool make gcc'

if (($UBUNTUVERSION < 16)); then
  echo "Your version of Ubuntu is not supported. Must have Ubuntu 16.04 and up. Aborting installation script..."
  exit 1
elif (($UBUNTUVERSION == 16)); then
  DISTRO='xenial'
  OS='ubuntu16.04-server'
elif (($UBUNTUVERSION == 18)); then
  DISTRO='bionic'
  OS='ubuntu18.04-server'
elif (($UBUNTUVERSION == 20)); then
  DISTRO='focal'
  OS='ubuntu20.04-server'
fi

echo "\n\n###############################################"
echo "#####       Installing Intel SGX driver       #####"
echo "###############################################\n\n"

# Download SGX driver
if (($UBUNTUVERSION == 16)); then
   # Ubuntu 16 was deprecated by the latest Intel SGX drivers
   wget "https://download.01.org/intel-sgx/sgx-linux/2.13/distro/${OS}/sgx_linux_x64_driver_2.11.0_0373e2e.bin"
else
   wget "https://download.01.org/intel-sgx/sgx-linux/2.14/distro/${OS}/sgx_linux_x64_driver_2.11.0_2d2b795.bin"
fi

# Make the driver installer executable
chmod +x ./sgx_linux_x64_driver_*.bin

# Install the driver
sudo ./sgx_linux_x64_driver_*.bin

# Remount /dev as exec, also at system startup
sudo tee /etc/systemd/system/remount-dev-exec.service >/dev/null <<EOF
[Unit]
Description=Remount /dev as exec to allow AESM service to boot and load enclaves into SGX

[Service]
Type=oneshot
ExecStart=/bin/mount -o remount,exec /dev
RemainAfterExit=true

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable remount-dev-exec
sudo systemctl start remount-dev-exec

echo "\n\n###############################################"
echo "#####       Installing Intel SGX PSW          #####"
echo "###############################################\n\n"

# Add Intels's SGX PPA
echo "deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu $DISTRO main" |
   sudo tee /etc/apt/sources.list.d/intel-sgx.list
wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key |
   sudo apt-key add -
sudo apt update

# Install libprotobuf
if (($UBUNTUVERSION > 18)); then
   sudo apt install -y gdebi
   # Install all the additional necessary dependencies (besides the driver and the SDK)
   # for building a rust enclave
   wget -O /tmp/libprotobuf30_3.19.4-1_amd64.deb https://engfilestorage.blob.core.windows.net/filestorage/libprotobuf30_3.19.4-1_amd64.deb
   yes | sudo gdebi /tmp/libprotobuf30_3.19.4-1_amd64.deb
else
   PSW_PACKAGES+=' libprotobuf-dev'
fi

sudo apt install -y $PSW_PACKAGES

# Add SGX_SDK, PKG_CONFIG_PATH and LD_LIBRARY_PATH directory into bashrc/zshrc files (unless disabled)
updaterc "$(cat << EOF
export SGX_SDK="/opt/sgxsdk"
export PATH="\${PATH}:/opt/sgxsdk/bin:/opt/sgxsdk/bin/x64"
export PKG_CONFIG_PATH="\${PKG_CONFIG_PATH}:/opt/sgxsdk/pkgconfig"
export LD_LIBRARY_PATH="\${LD_LIBRARY_PATH}:/opt/sgxsdk/sdk_libs"
EOF
)"