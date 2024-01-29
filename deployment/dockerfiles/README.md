The build process for SN is a bit complex, and can include some unexpected parts for those that have not been baptised in the waters of TEEs.

### Build Targets Overview
1. **`release-image`**: Creates a full node Docker image.
2. **`build-deb`**: Generates a Debian (.deb) package
3. **`build-deb-mainnet`**: Similar to `build-deb`, but specifically for generating a Debian package for mainnet.
4. **`compile-secretd`**: Produces an image with the compiled enclave and `secretd`, a core component of the Secret Network - sometimes you don't need the whole image, just secretd.

### Build Process Description

#### 1. **Base Images and Environment Setup**
- Defines two base images: 
  * `SCRT_BASE_IMAGE_ENCLAVE` - Used as the base for building the enclave components, which are crucial for the secure execution of code in an SGX (Software Guard Extensions) environment.  
  * `SCRT_RELEASE_BASE_IMAGE` - Serves as the base for the final release image that contains all the necessary components to run a full node.
- Sets up the environment for subsequent stages, including installing dependencies like `clang-10`, setting `WORKDIR`, and other environment variables.

#### 2. **Compilation of Enclaves**
- **`prepare-compile-enclave` & `compile-enclave`**: Prepares the environment and compiles the enclaves.
- **`compile-tendermint-enclave`**: Compiles the Tendermint enclave, which is a part of the blockchain consensus mechanism.

#### 3. **Compilation of `secretd`**
- Sets up the Go environment and downloads specific Go packages.
- Copies source files and prepares the environment for building `secretd`.
- Uses the compiled enclaves from previous steps.

#### 4. **Release Image Creation (`release-image`)**
- Creates the final node image with all necessary binaries and libraries.
- Installs additional dependencies like `jq`, `openssl`, and Node.js - these are used for the faucet and for debugging tools.
- Sets up environment variables and links libraries.

#### 5. **Mainnet Upgrade (`mainnet-release`)**
- Upgrades the `release-image` with specific binaries and libraries for the mainnet.

#### 6. **Debian Package Creation (`build-deb` and `build-deb-mainnet`)**
- Prepares an environment for building Debian packages.
- Copies necessary binaries and libraries from previous stages.
- Executes a script to build the Debian package.

#### 7. **Compilation of `check-hw` Tool (`compile-check-hw-tool`)**
- Compiles a hardware check tool, necessary for validating the hardware running the Secret Network nodes - this is unrelated to the release image or the network node directly.

#### 8. **LocalSecret Setup (`build-localsecret`)**
- A specialized setup for a local version of the Secret Network, including a faucet server and a health check mechanism for local development.

### Summary
Each target in this Dockerfile serves a distinct purpose in the build and deployment pipeline of the Secret Network. From compiling essential components like `secretd` and the Tendermint enclave, to packaging these components for deployment in various environments (development, mainnet), the Dockerfile covers a comprehensive range of tasks necessary for maintaining and deploying a blockchain network. The use of multi-stage builds optimizes the process by reusing stages and minimizing the final image size.
