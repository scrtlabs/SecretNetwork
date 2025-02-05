# Build with:
# docker build -f hermes.Dockerfile . -t hermes:latest
# FROM golang:latest
FROM ubuntu:latest

# add hermes user
RUN ["useradd", "-ms", "/bin/bash", "hermes-user"]
USER hermes-user 
WORKDIR /home/hermes-user

# install hermes
ADD ["./hermes-v1.10.3-x86_64-unknown-linux-gnu.tar.gz", "/hermes-installation"]
ENV PATH="${PATH}:/hermes-installation"

# configure hermes
COPY --chown=hermes-user ["./hermes-config.toml", "/home/hermes-user/.hermes/config.toml"]
COPY --chown=hermes-user ["./hermes-alternative-config.toml", "/home/hermes-user/.hermes/alternative-config.toml"]

# add keys on both chains
COPY ["./d.mnemonic", "/home/hermes-user/d.mnemonic"]
RUN ["hermes", "keys", "add", "--hd-path", "m/44'/529'/0'/0/0", "--mnemonic-file", "/home/hermes-user/d.mnemonic", "--chain", "secretdev-1"]
RUN ["hermes", "keys", "add", "--hd-path", "m/44'/529'/0'/0/0", "--mnemonic-file", "/home/hermes-user/d.mnemonic", "--chain", "secretdev-2"]

# add alternative key
COPY ["./c.mnemonic", "/home/hermes-user/c.mnemonic"]
RUN ["hermes", "keys", "add", "--hd-path", "m/44'/529'/0'/0/0", "--mnemonic-file", "/home/hermes-user/c.mnemonic", "--chain", "secretdev-1", "--key-name", "local1-alt"]
RUN ["hermes", "keys", "add", "--hd-path", "m/44'/529'/0'/0/0", "--mnemonic-file", "/home/hermes-user/c.mnemonic", "--chain", "secretdev-2", "--key-name", "local2-alt"]

# start hermes
COPY --chown=hermes-user ["./entrypoint-hermes.sh", "/home/hermes-user/entrypoint.sh"]
RUN  ["chmod", "+x", "/home/hermes-user/entrypoint.sh"]
ENTRYPOINT ["/home/hermes-user/entrypoint.sh"]
