# Verify your SGX setup

# Background

To make sure the entire network runs on SGX nodes we use a process called _registration_. This process, performed by each node runner, involves authenticating the local enclave both with Intel Attestation Services and on-chain. 

This process not only verifies that the local node is running a genuine enclave, but that it is patched, and not vulnerable to any known exploits. This means that you may be running SGX-enabled hardware, but may be missing microcode, or firmware which affect SGX-security. 

For this reason it is recommended to check ahead of time the result of the attestation process, which can tell you if an update is required.

__Note:__ for the incentivized testnet we are going to run with more relaxed requirements than mainnet - be aware that your incentivized testnet setup may not work on mainnet if you do not verify it

## Instructions

These instructions refer to an installation using:
* Ubuntu 18.04 or 20.04
* SGX driver [sgx_linux_x64_driver_2.6.0_95eaa6f.bin](https://download.01.org/intel-sgx/sgx-linux/2.9.1/distro/ubuntu18.04-server/sgx_linux_x64_driver_2.6.0_95eaa6f.bin "sgx_linux_x64_driver_2.6.0_95eaa6f.bin")
* Intel SGX PSW 2.9.101.2

See SGX installation instructions [here](https://github.com/enigmampc/SecretNetwork/blob/develop/docs/dev/setup-sgx.md)

Other driver/OS combinations are not guaranteed to work with these instructions. Let us know on `chat.scrt.network` if you intend to run on a different setup.

### 1. Download the test package

`wget https://github.com/enigmampc/SecretNetwork/releases/download/v0.5.0-alpha2/secretnetwork_0.5.0-alpha2_amd64.deb`

### 2. Unpack 
#### This will install `secretd`
`sudo dpkg -i secretnetwork_0.5.0-alpha2_amd64.deb`

### 3. Initialize the enclave
`secretd init-enclave`

This step, if successful, will create an output similar to this - 
```
DEBUG [wasmi_runtime_enclave::registration::report] Certificate verified
DEBUG [wasmi_runtime_enclave::registration::report] Signature verified successfully  
DEBUG [wasmi_runtime_enclave::registration::report] attn_report: {"advisoryIDs":["INTEL-SA-00334"],"advisoryURL":"https://securitycenter.intel.com","id":"224284791301521900462648270992134615927","isvEnclaveQuoteBody":"AgAAAMYLAAALAAoAAAAAABf93MlHcUSizYTifNzpi+RVVKsDsv2Ja81QMM7E0QebDw8DBf+ABgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAABwAAAAAAAAAHAAAAAAAAAO40oSBNM9hG2mHlwrSwAbMCmy87uJSAiGpez88uqJlgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACD1xnnferKF
HD2uvYqTXdDA8iZ22kCD5xw7h38CMfOngAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAC+GK4P6LTvaY3qvEQ2Bnje5LxZtKDaki4iEKSU9FjmCwAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
"isvEnclaveQuoteStatus":"SW_HARDENING_NEEDED", "timestamp":"2020-07-12T09:08:33.572985","version":4}
```

Where the import field is __isvEnclaveQuoteStatus__. This is the field that marks trust level of our platform. The acceptable values for this field are:

* OK
* SW_HARDENING_NEEDED

With the following value accepted for __incentivized testnet only__:
* GROUP_OUT_OF_DATE

Consult with the [Intel API](https://api.trustedservices.intel.com/documents/sgx-attestation-api-spec.pdf#page=21) for more on these values.

If you do not see such an output, look for a file called `attestation_cert.der` which should have been created in your $(home) directory.  You can then use the command `secretd parse <path/to/attestation_cert.der>` to check the result a successful result should be a 64 byte hex string (e.g. `0x9efe0dc689447514d6514c05d1161cea15c461c62e6d72a2efabcc6b85ed953b`. 

### 4. What to do if this didn't work?

1. Running `secretd init-enclave` should have created a file called `attestation_cert.der`. This file contains the attestation report from above.
2. Contact us on the proper channels on `chat.scrt.network`
3. The details we will need to investigate will include:
	* Hardware specs
	* SGX PSW/driver versions
	* BIOS versions
	* The file `attestation_cert.der`
