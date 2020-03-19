# Hardware

1. Go to your BIOS menu
2. Enable SGX (Software controlled is not enough)
3. Disable Secure Boot

# Software

This script was tested on Ubuntu 20.04 with SGX driver/sdk version 2.9 intended for Ubuntu 18.04:

```bash
# Create a working directory to download and install the SDK inside
mkdir -p "$HOME/.sgxsdk"

(
   # In a new sub-shell cd into our working directory so to no pollute the
   # original shell's working directory
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
   echo "source '$HOME/.sgxsdk/sgxsdk/environment'" |
      tee -a "$HOME/.bashrc" "$HOME/.zshrc" > /dev/null
)

# Add Intels's SGX PPA
echo 'deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu bionic main' |
   sudo tee /etc/apt/sources.list.d/intel-sgx.list
wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key |
   sudo apt-key add -
sudo apt update

# Install all the additional necessary dependencies (besides the driver and the SDK)
# for building a rust enclave
wget -O /tmp/libprotobuf10_3.0.0-9_amd64.deb http://ftp.br.debian.org/debian/pool/main/p/protobuf/libprotobuf10_3.0.0-9_amd64.deb
(sleep 3 ; echo y) | sudo gdebi /tmp/libprotobuf10_3.0.0-9_amd64.deb

sudo apt install -y libsgx-enclave-common libsgx-enclave-common-dev libsgx-urts sgx-aesm-service libsgx-uae-service libsgx-launch autoconf libtool
```

### Test that it works

```bash
sudo apt install -y libssl-dev protobuf-compiler
cargo +nightly install fortanix-sgx-tools sgxs-tools

sgx-detect
```

```bash
git clone --depth 1 -b v1.1.1-testing git@github.com:apache/incubator-teaclave-sgx-sdk.git

cd incubator-teaclave-sgx-sdk/samplecode/hello-rust
perl -i -pe 's/SGX_SDK \?=.+/SGX_SDK ?= \$(HOME)\/sgxsdk\/.sgxsdk/' Makefile
make
cd bin
./app
```

   ```bash
   git clone --depth 1 -b v1.1.1-testing git@github.com:apache/incubator-teaclave-sgx-sdk.git

   cd incubator-teaclave-sgx-sdk/samplecode/hello-rust
   perl -i -pe 's/SGX_SDK \?=.+/SGX_SDK ?= \$(HOME)\/.sgxsdk\/sgxsdk/' Makefile
   make
   cd bin
   ./app
   ```

   Should print somting similar to this:

   ```
   [+] Init Enclave Successful 2!
   This is a normal world string passed into Enclave!
   This is a in-Enclave Rust string!
   gd: 1 0 0 1
   static: 1 eremove: 0 dyn: 0
   EDMM: 0, feature: 9007268790009855
   supported sgx
   [+] say_something success...
   ```

# Uninstall

To uninstall the Intel(R) SGX Driver, run:

```bash
sudo /opt/intel/sgxdriver/uninstall.sh
```

The above command produces no output when it succeeds. If you want to verify that the driver has been uninstalled, you can run the following, which should print `SGX Driver NOT installed`:

```bash
ls /dev/isgx &>/dev/null && echo "SGX Driver installed" || echo "SGX Driver NOT installed"
```

To uninstall the SGX SDK, run:

```bash
sudo "$HOME"/.sgxsdk/sgxsdk/uninstall.sh
rm -rf "$HOME/.sgxsdk"
```

To uninstall the rest of the dependencies, run:

```bash
sudo apt purge -y libsgx-enclave-common libsgx-enclave-common-dev libsgx-urts sgx-aesm-service libsgx-uae-service libsgx-launch libsgx-aesm-launch-plugin libsgx-ae-le
```

# Refs

1. https://github.com/apache/incubator-teaclave-sgx-sdk/wiki/Environment-Setup
2. https://github.com/openenclave/openenclave/blob/master/docs/GettingStartedDocs/install_oe_sdk-Ubuntu_18.04.md
3. https://github.com/apache/incubator-teaclave-sgx-sdk/blob/783f04c002e243d1022c5af8a982f9c2a7138f32/dockerfile/Dockerfile.1804.nightly
4. https://edp.fortanix.com/docs/installation/guide/
