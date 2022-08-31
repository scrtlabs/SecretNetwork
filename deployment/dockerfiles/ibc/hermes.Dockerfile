# Build with:
# docker build -f hermes.Dockerfile . -t hermes:latest
# FROM golang:latest
FROM ubuntu:latest

# add hermes user
RUN ["useradd", "-ms", "/bin/bash", "hermes-user"]
USER hermes-user 
WORKDIR /home/hermes-user

# install hermes
ADD ["./hermes-v1.0.0-rc.2-x86_64-unknown-linux-gnu.tar.gz", "/hermes-installation"]
ENV PATH="${PATH}:/hermes-installation"

# configure hermes
COPY --chown=hermes-user ["./hermes-config.toml", "/home/hermes-user/.hermes/config.toml"]

# add keys on both chains
COPY ["./50s03.mnemonic", "/home/hermes-user/50s03.mnemonic"]
RUN ["hermes", "keys", "add", "--hd-path", "m/44'/529'/0'/0/0", "--mnemonic-file", "/home/hermes-user/50s03.mnemonic", "--chain", "secretdev-1"]
RUN ["hermes", "keys", "add", "--hd-path", "m/44'/529'/0'/0/0", "--mnemonic-file", "/home/hermes-user/50s03.mnemonic", "--chain", "secretdev-2"]

# start hermes
COPY --chown=hermes-user ["./entrypoint-hermes.sh", "/home/hermes-user/entrypoint.sh"]
RUN  ["chmod", "+x", "/home/hermes-user/entrypoint.sh"]
ENTRYPOINT ["/home/hermes-user/entrypoint.sh"]
