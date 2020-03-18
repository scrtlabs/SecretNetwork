# Hardware

1. Go to your BIOS menu
2. Enable SGX (Software controlled is not enough)
3. Disable Secure Boot

# Software

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
ls /dev/isgx &>/dev/null && echo "SGX Driver installed" || echo "SGX Driver NOT installed"

(echo no && sleep 0.5 && echo "$HOME/.sgxsdk") | ./sgx_linux_x64_sdk_*.bin
```

# Testing your SGX setup

1. For node runners, by using `sgx-detect`:

   ```bash
   sudo apt install -y libssl-dev protobuf-compiler
   cargo +nightly install fortanix-sgx-tools sgxs-tools

   sgx-detect
   ```

   Should print at the end:

   ```
   ✔  Able to launch enclaves
      ✔  Debug mode
      ✔  Production mode (Intel whitelisted)

   You're all set to start running SGX programs!
   ```

5. `chmod +x sgx_linux_*.bin`

   ```bash
   git clone --depth 1 -b v1.1.1-testing git@github.com:apache/incubator-teaclave-sgx-sdk.git

   Verify that the driver is installed correctly:

   ```bash
   ls /dev/isgx &>/dev/null && echo "SGX Driver installed" || echo "SGX Driver NOT installed"
   ```

7. `./sgx_linux_x64_sdk_*.bin`

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
