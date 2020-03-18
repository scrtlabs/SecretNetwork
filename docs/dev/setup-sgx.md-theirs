# Hardware

TODO

# Software

Notes: These commands can replace steps 1-7:  
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

1. Go to https://download.01.org/intel-sgx/sgx-linux

2. Step into the latest version

3. Step into `distro/$LATEST_UBUNTU_YOU_SEE_THERE`

4. Download `sgx_linux_x64_driver_*.bin` and `sgx_linux_x64_sdk_*.bin`

5. `chmod +x sgx_linux_*.bin`

6. `sudo ./sgx_linux_x64_driver_*.bin`

   Verify that the driver is installed correctly:

   ```bash
   ls /dev/isgx &>/dev/null && echo "SGX Driver installed" || echo "SGX Driver NOT installed"
   ```

7. `./sgx_linux_x64_sdk_*.bin`

8. At the end of the previous step you should have received a command to run (E.g. `source $HOME/.sgxsdk/environment`) - add it to your `.bashrc` or `.zshrc`.

9. ```bash
   echo 'deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu bionic main' | sudo tee /etc/apt/sources.list.d/intel-sgx.list
   wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | sudo apt-key add -
   sudo apt install -y libsgx-enclave-common libsgx-enclave-common-dev
   ```

TODO: Add steps on how the test the setup (E.g. compiling & running a helloworld program)

# Refs

1. https://github.com/apache/incubator-teaclave-sgx-sdk/wiki/Environment-Setup
2. https://github.com/openenclave/openenclave/blob/master/docs/GettingStartedDocs/install_oe_sdk-Ubuntu_18.04.md
