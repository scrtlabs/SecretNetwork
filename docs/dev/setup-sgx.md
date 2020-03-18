# Hardware

TODO

# Software

<<<<<<< HEAD
This script was tested on Ubuntu 20.04 with SGX driver/sdk version 2.9 intended for Ubuntu 18.04:

```bash
# Create a working directory to download and install the SDK inside
mkdir -p "$HOME/.sgxsdk"

(
   # In a new sub-shell cd into our working directory so to no pollute the original shell's working directory
   cd "$HOME/.sgxsdk"

   # 1. Go to https://download.01.org/intel-sgx/sgx-linux
   # 2. Step into the latest version
   # 3. Step into `distro/$LATEST_UBUNTU_YOU_SEE_THERE`
   # 4. Download `sgx_linux_x64_driver_*.bin` and `sgx_linux_x64_sdk_*.bin`
   lynx -dump -listonly -nonumbers https://download.01.org/intel-sgx/sgx-linux/ |
      grep -P 'sgx-linux/(\d\.?)+/' |
      sort |
      tail -1 |
      parallel --bar --verbose lynx -dump -listonly -nonumbers "{}/distro" |
      grep -P 'ubuntu\d\d' |
      sort |
      tail -1 |
      parallel --bar --verbose lynx -dump -listonly -nonumbers |
      grep -P '\.bin$' |
      parallel --bar --verbose curl -OSs

   # Make the driver and SDK installers executable
   chmod +x ./sgx_linux_*.bin

   # Install the driver
   sudo ./sgx_linux_x64_driver_*.bin

   # Verify SGX driver installation
   ls /dev/isgx &>/dev/null && echo "SGX Driver installed" || echo "SGX Driver NOT installed"

   # Install the SDK inside ./sgxsdk which is inside $HOME/.sgxsdk
   echo yes | ./sgx_linux_x64_sdk_*.bin

   # Setup the environment variables for every new shell
   echo "source '$HOME/.sgxsdk/sgxsdk/environment'" | tee -a "$HOME/.bashrc" "$HOME/.zshrc" > /dev/null
)

# Add
echo 'deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu bionic main' |
   sudo tee /etc/apt/sources.list.d/intel-sgx.list
wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key |
   sudo apt-key add -

# Install all the additional necessary dependencies (besides the driver and the SDK) for building a rust enclave
sudo apt install -y libsgx-enclave-common libsgx-enclave-common-dev autoconf
=======
Note: These commands can replace steps 1-7:  
(Tested with version 2.9 and Ubuntu 18.04)

```bash
lynx -dump -listonly -nonumbers https://download.01.org/intel-sgx/sgx-linux/ |
    grep -P 'sgx-linux/(\d\.?)+/' |
    sort |
    tail -1 |
    parallel --bar --verbose lynx -dump -listonly -nonumbers "{}/distro" |
    grep -P 'ubuntu\d\d' |
    sort |
    tail -1 |
    parallel --bar --verbose lynx -dump -listonly -nonumbers |
    grep -P '\.bin$' |
    parallel --bar --verbose curl -OSs

chmod +x *.bin
sudo ./sgx_linux_x64_driver_*.bin
(echo no && sleep 0.1 && echo "$HOME/.sgxsdk") | ./sgx_linux_x64_sdk_*.bin
>>>>>>> 4c5a84aa5ed38e4c46ced79cbca3635271e6629b
```

TODO: Add steps on how the test the setup (E.g. compiling & running a helloworld program)

# Refs

1. https://github.com/apache/incubator-teaclave-sgx-sdk/wiki/Environment-Setup
2. https://github.com/openenclave/openenclave/blob/master/docs/GettingStartedDocs/install_oe_sdk-Ubuntu_18.04.md
