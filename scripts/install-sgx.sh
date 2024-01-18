#! /bin/bash

set -e

INSTALL_DEPS=${1:-"true"}
INSTALL_SDK=${2:-"true"}
INSTALL_PSW=${3:-"true"}
INSTALL_DRIVER=${4:-"true"}
UBUNTUVERSION=$(grep 'VERSION_ID' /etc/os-release | cut -d '"' -f 2 | cut -d '.' -f 1)

# Check for root privileges
if [ "$(id -u)" -ne 0 ]; then
    echo -e 'Script must be run as root. Use sudo, su, or add "USER root" to your Dockerfile before running this script.'
    exit 1
fi

# Check if the Ubuntu version is supported
if (($UBUNTUVERSION < 16)); then
    echo "Your version of Ubuntu is not supported. Must have Ubuntu 16.04 and up. Aborting installation script..."
    exit 1
elif (($UBUNTUVERSION < 18)); then
    DISTRO='xenial'
elif (($UBUNTUVERSION < 20)); then
    DISTRO='bionic'
    OS='ubuntu18.04-server'
elif (($UBUNTUVERSION < 22)); then
    DISTRO='focal'
    OS='ubuntu20.04-server'
else
    DISTRO='jammy'
    OS='ubuntu22.04-server'
fi

# Function to install missing packages
deps() {
    echo "\n\n#######################################"
    echo "##### Installing missing packages #####"
    echo "#######################################\n\n"

    apt-get update
    apt-get install -y make wget
}

# Function to install the SDK
install_sdk(){
    echo "\n\n############################################"
    echo "##### Installing Intel SGX SDK #####"
    echo "############################################\n\n"

    mkdir -p "$HOME/.sgxsdk"
    cd "$HOME/.sgxsdk"

    wget -O sgx_linux_x64_sdk_2.17.101.1.bin "https://download.01.org/intel-sgx/sgx-linux/2.17.1/distro/ubuntu20.04-server/sgx_linux_x64_sdk_2.17.101.1.bin"
    chmod +x ./sgx_linux_x64_sdk_*.bin
    (echo no; echo /opt/intel/) | ./sgx_linux_x64_sdk_2.17.101.1.bin
    echo "source '/opt/intel/sgxsdk/environment'" | tee -a "$HOME/.bashrc" "$HOME/.zshrc" > /dev/null
}

# Function to install the SGX driver
install_sgx_driver(){
    echo "\n\n###############################################"
    echo "##### Installing Intel SGX driver #####"
    echo "###############################################\n\n"

    wget -O sgx_linux_x64_driver_2.11.0_0373e2e.bin "https://download.01.org/intel-sgx/sgx-linux/2.13/distro/ubuntu20.04-server/sgx_linux_x64_driver_2.11.0_0373e2e.bin"
    chmod +x ./sgx_linux_x64_driver_*.bin
    ./sgx_linux_x64_driver_*.bin

    # Configure the system to remount /dev as exec on boot
    cat > /etc/systemd/system/remount-dev-exec.service <<EOF
[Unit]
Description=Remount /dev as exec to allow AESM service to boot and load enclaves into SGX

[Service]
Type=oneshot
ExecStart=/bin/mount -o remount,exec /dev
RemainAfterExit=true

[Install]
WantedBy=multi-user.target
EOF

    systemctl enable remount-dev-exec
    systemctl start remount-dev-exec
}

# Function to install Platform Software (PSW)
install_psw(){
    echo "\n\n##############################################"
    echo "##### Installing Intel SGX PSW #####"
    echo "##############################################\n\n"

    apt-get update
    apt-get install -y gdebi-core --no-install-recommends
    echo "deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu $DISTRO main" | tee /etc/apt/sources.list.d/intel-sgx.list
    wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | apt-key add -
    apt-get update
    apt-get install -y libsgx-enclave-common libsgx-enclave-common-dev libsgx-urts sgx-aesm-service libsgx-uae-service libsgx-launch libsgx-aesm-launch-plugin libsgx-ae-le sgx-aesm-service libsgx-uae-service autoconf libtool
}

# Install dependencies if requested
[ "${INSTALL_DEPS}" = "true" ] && deps

# Install SDK if requested
[ "${INSTALL_SDK}" = "true" ] && install_sdk

# Install PSW if requested
[ "${INSTALL_PSW}" = "true" ] && install_psw

# Install SGX driver if requested
[ "${INSTALL_DRIVER}" = "true" ] && install_sgx_driver

echo -e "\nInstallation process completed."
