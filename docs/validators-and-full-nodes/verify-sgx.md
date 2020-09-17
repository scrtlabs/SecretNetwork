# Verify your SGX setup

# Background

To make sure the entire network runs on SGX nodes we use a process called _registration_. This process, performed by each node runner, involves authenticating the local enclave both with Intel Attestation Services and on-chain.

This process not only verifies that the local node is running a genuine enclave, but that it is patched, and not vulnerable to any known exploits. This means that you may be running SGX-enabled hardware, but may be missing microcode, or firmware which affect SGX-security.

For this reason it is recommended to check ahead of time the result of the attestation process, which can tell you if an update is required.

**Note:** for the incentivized testnet we are going to run with more relaxed requirements than mainnet - be aware that your incentivized testnet setup may not work on mainnet if you do not verify it

## Instructions

These instructions refer to an installation using:

- Ubuntu 18.04 or 20.04
- SGX driver [sgx_linux_x64_driver_2.6.0_95eaa6f.bin](https://download.01.org/intel-sgx/sgx-linux/2.9.1/distro/ubuntu18.04-server/sgx_linux_x64_driver_2.6.0_95eaa6f.bin "sgx_linux_x64_driver_2.6.0_95eaa6f.bin")
- Intel SGX PSW 2.9.101.2

See SGX installation instructions [here](../validators-and-full-nodes/setup-sgx.md)

Other driver/OS combinations are not guaranteed to work with these instructions. Let us know on `chat.scrt.network` if you intend to run on a different setup.

### 1. Download the test package

`wget https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.0/secretnetwork_1.0.0_amd64.deb`

### 2. Unpack

#### This will install `secretd`

```bash
sudo dpkg -i secretnetwork_1.0.0_amd64.deb
```

### 3. Initialize the enclave

Create the `.sgx_secrets` directory if it doesn't already exist

```bash
mkdir .sgx_secrets
```

Then initialize the enclave

```bash
SCRT_ENCLAVE_DIR=/usr/lib secretd init-enclave
```

(or `SCRT_ENCLAVE_DIR=/usr/lib secretd init-enclave | grep -Po 'isvEnclaveQuoteStatus":".+?"'`)

This step, if successful, will create an output similar to this -

```
INFO  [wasmi_runtime_enclave::registration::attestation] Attestation report: {"id":"183845695958032083367610248637243990718","timestamp":"2020-07-12T09:43:12.297820","version":4,"advisoryURL":"https://security-center.intel.com","advisoryIDs":["INTEL-SA-00334"],"isvEnclaveQuoteStatus":"SW_HARDENING_NEEDED","isvEnclaveQuoteBody":"AgAAAMYLAAALAAoAAAAAABf93MlHcUSizYTifNzpi+QD9Lqdmd+k62/B9e4nOc4sDw8DBf+ABgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABwAAAAAAAAAHAAAAAAAAAO40oSBNM9hG2mHlwrSwAbMCmy87uJSAiGpez88uqJlgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACD1xnnferKFHD2uvYqTXdDA8iZ22kCD5xw7h38CMfOngAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADv56xwUqy2HPiT/uxTSwg1LQmFPJa2sD0Q2YwuzlJuLgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}
```

(or `isvEnclaveQuoteStatus":"SW_HARDENING_NEEDED"`)

Where the important fields are **isvEnclaveQuoteStatus** and **advisoryIDs**. This is are fields that mark the trust level of our platform. The acceptable values for the `isvEnclaveQuoteStatus` field are:

- OK
- SW_HARDENING_NEEDED

With the following value accepted for **testnet only**:

- GROUP_OUT_OF_DATE

For the status `CONFIGURATION_AND_SW_HARDENING_NEEDED` we perform a deeper inspection of the exact vulnerabilities that remain. The acceptable values **for mainnet** are:

- `"INTEL-SA-00334"`
- `"INTEL-SA-00219"`

Consult with the [Intel API](https://api.trustedservices.intel.com/documents/sgx-attestation-api-spec.pdf#page=21) for more on these values.

If you do not see such an output, look for a file called `attestation_cert.der` which should have been created in your `$HOME` directory. You can then use the command `secretd parse <path/to/attestation_cert.der>` to check the result a successful result should be a 64 byte hex string (e.g. `0x9efe0dc689447514d6514c05d1161cea15c461c62e6d72a2efabcc6b85ed953b`.

### 4. What to do if this didn't work?

1. Running `secretd init-enclave` should have created a file called `attestation_cert.der`. This file contains the attestation report from above.
2. Contact us on the proper channels on `chat.scrt.network`
3. The details we will need to investigate will include:
   - Hardware specs
   - SGX PSW/driver versions
   - BIOS versions
   - The file `attestation_cert.der`

### 5. Troubleshooting

#### Output is:

```
secretd init-enclave
2020-07-12 13:21:31,864 ERROR [go_cosmwasm] Error :(
ERROR: failed to initialize enclave: Error calling the VM: SGX_ERROR_ENCLAVE_FILE_ACCESS
```

Make sure you have the environment variable `SCRT_ENCLAVE_DIR=/usr/lib` set before you run `secretd`.

#### Output is:

```
secretd init-enclave
ERROR  [wasmi_runtime_enclave::crypto::key_manager] Error sealing registration key
ERROR  [wasmi_runtime_enclave::registration::offchain] Failed to create registration key
2020-07-12 13:37:26,690 ERROR [go_cosmwasm] Error :(
ERROR: failed to initialize enclave: Error calling the VM: SGX_ERROR_UNEXPECTED
```

Make sure the directory `~/.sgx_secrets/` is created. If that still doesn't work, try to create `/root/.sgx_secrets`

#### Output is:

```
secretd init-enclave
ERROR  [wasmi_runtime_enclave::registration::attestation] Error in create_attestation_report: SGX_ERROR_SERVICE_UNAVAILABLE
ERROR  [wasmi_runtime_enclave::registration::offchain] Error in create_attestation_certificate: SGX_ERROR_SERVICE_UNAVAILABLE
ERROR: failed to create attestation report: Error calling the VM: SGX_ERROR_SERVICE_UNAVAILABLE
```

Make sure the `aesmd-service` is running `systemctl status aesmd.service`

#### I'm seeing `CONFIGURATION_AND_SW_HARDENING_NEEDED` in the `isvEnclaveQuoteStatus` field, but with more advisories than what is allowed

This could mean a number of different things related to the configuration of the machine. Most common are:

- ["INTEL-SA-00161", "INTEL-SA-00233"] - Hyper-threading must be disabled in the BIOS
- ["INTEL-SA-00289"] - Overclocking/undervolting must be disabled by the BIOS
- ["INTEL-SA-00219"] - Integrated graphics should be disabled in the BIOS - we recommend performing this step if you can, though it isn't required

#### I'm seeing `SGX_ERROR_DEVICE_BUSY`

Most likely you tried reinstalling the driver and rerunning the enclave - restarting should solve the problem
