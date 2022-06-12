variable "VERSION" {
  default = "v1.3.1"
}

target "base" {
  dockerfile = "deployment/dockerfiles/base.Dockerfile"
  args = {
    SGX_MODE="SW",
    FEATURES="debug-print",
    BUILD_VERSION="${VERSION}"
  }
  platforms = ["linux/arm64", "linux/amd64"]
}

target "build-release" {
  dockerfile = "deployment/dockerfiles/release.Dockerfile"
  contexts = {
    rust-go-base-image = "target:base"
  }
  args = {
    SGX_MODE="SW",
    SECRET_NODE_TYPE="BOOTSTRAP",
    CHAIN_ID="secretdev-1"
  }
  platforms = ["linux/arm64", "linux/amd64"]
}

target "build-dev-image" {
  dockerfile = "deployment/dockerfiles/dev-image.Dockerfile"
  contexts = {
    build-release = "target:build-release"
  }
  args = {
    SGX_MODE="SW",
    SECRET_NODE_TYPE="BOOTSTRAP",
    CHAIN_ID="secretdev-1"
  }
  platforms = ["linux/arm64", "linux/amd64"]
  output = [
    "type=registry"
  ]
  tags = ["ghcr.io/scrtlabs/localsecret:${VERSION}"]
}