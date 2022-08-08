#! /bin/bash

set -e

INSTALL_DEPS=${1:-"true"}
INSTALL_SDK=${2:-"true"}
INSTALL_PSW=${3:-"true"}
INSTALL_DRIVER=${4:-"true"}
UBUNTUVERSION=$(lsb_release -r -s | cut -d '.' -f 1)

if [ "$(id -u)" -ne 0 ]; then
    echo -e 'Script must be run as root. Use sudo, su, or add "USER root" to your Dockerfile before running this script.'
    exit 1
fi

if (($UBUNTUVERSION < 16)); then
        echo "Your version of Ubuntu is not supported. Must have Ubuntu 16.04 and up. Aborting installation script..."
        exit 1
elif (($UBUNTUVERSION < 18)); then
        DISTRO='xenial'
elif (($UBUNTUVERSION < 20)); then
        DISTRO='bionic'
        OS='ubuntu18.04-server'
else
        DISTRO='focal'
        OS='ubuntu20.04-server'
fi

deps() {
    echo "\n\n#######################################"
    echo "##### Installing missing packages #####"
    echo "#######################################\n\n"

    # Install needed packages for script
    sudo apt install -y make wget
}

install_sdk(){
   # Create a working directory to download and install the SDK inside
   mkdir -p "$HOME/.sgxsdk"

   # In a new sub-shell cd into our working directory so to no pollute the
   # original shell's working directory
   cd "$HOME/.sgxsdk"

   wget -O sgx_linux_x64_sdk_2.13.100.4.bin https://download.01.org/intel-sgx/sgx-linux/2.13/distro/ubuntu20.04-server/sgx_linux_x64_sdk_2.13.100.4.bin

   # Make the driver and SDK installers executable
   chmod +x ./sgx_linux_*.bin

   # Install the SDK in /opt/intel/sgxsdk
   (echo no; echo /opt/intel/) | ./sgx_linux_x64_sdk_2.13.100.4.bin

   # Setup the environment variables for every new shell
   echo "source '/opt/intel/.sgxsdk/sgxsdk/environment'" |
      tee -a "$HOME/.bashrc" "$HOME/.zshrc" > /dev/null
}

install_sgx_driver(){
   echo "\n\n###############################################"
   echo "##### Installing Intel SGX driver #####"
   echo "###############################################\n\n"

   wget -O sgx_linux_x64_driver_2.11.0_0373e2e.bin https://download.01.org/intel-sgx/sgx-linux/2.13/distro/ubuntu20.04-server/sgx_linux_x64_driver_2.11.0_0373e2e.bin

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


}

install_psw(){
    echo "\n\n##############################################"
    echo "##### Installing additional dependencies #####"
    echo "##############################################\n\n"

    # Install needed packages for script
    sudo apt update
    sudo apt install -y gdebi --no-install-recommends

    # Add Intel's SGX PPA
    echo "deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu $DISTRO main" |
    sudo tee /etc/apt/sources.list.d/intel-sgx.list
    wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key |
    sudo apt-key add -
    sudo apt update

    # Install all the additional necessary dependencies (besides the driver and the SDK)
    # for building a rust enclave
    wget -O /tmp/libprotobuf10_3.0.0-9_amd64.deb http://ftp.br.debian.org/debian/pool/main/p/protobuf/libprotobuf10_3.0.0-9_amd64.deb
    (sleep 3 ; echo y) | sudo gdebi /tmp/libprotobuf10_3.0.0-9_amd64.deb

    sudo apt install -y libsgx-enclave-common libsgx-enclave-common-dev libsgx-urts sgx-aesm-service libsgx-uae-service libsgx-launch libsgx-aesm-launch-plugin libsgx-ae-le autoconf libtool
}

deps

if [ "${INSTALL_SDK}" = "true" ]; then
   install_sdk
fi

if [ "${INSTALL_PSW}" = "true" ]; then
   install_psw
fi

if [ "${INSTALL_DRIVER}" = "true" ]; then
   install_sgx_driver
fi
