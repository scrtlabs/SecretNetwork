# Final image
FROM build-release

ARG SGX_MODE=SW
ENV SGX_MODE=${SGX_MODE}
#
ARG SECRET_LOCAL_NODE_TYPE
ENV SECRET_LOCAL_NODE_TYPE=${SECRET_LOCAL_NODE_TYPE}

ENV PKG_CONFIG_PATH=""
ENV SCRT_ENCLAVE_DIR=/usr/lib/

COPY deployment/docker/sanity-test.sh /root/
RUN chmod +x /root/sanity-test.sh

COPY x/compute/internal/keeper/testdata/erc20.wasm erc20.wasm
RUN true
COPY deployment/ci/wasmi-sgx-test.sh .
RUN true
COPY deployment/ci/bootstrap_init.sh .
RUN true
COPY deployment/ci/node_init.sh .
RUN true
COPY deployment/ci/startup.sh .
RUN true
COPY deployment/ci/node_key.json .

RUN chmod +x /usr/bin/secretd
# RUN chmod +x /usr/bin/secretcli
RUN chmod +x wasmi-sgx-test.sh
RUN chmod +x bootstrap_init.sh
RUN chmod +x startup.sh
RUN chmod +x node_init.sh


#RUN mkdir -p /root/.secretd/.compute/
#RUN mkdir -p /root/.sgx_secrets/
#RUN mkdir -p /root/.secretd/.node/

# Enable autocomplete
#RUN secretcli completion > /root/secretcli_completion
#RUN secretd completion > /root/secretd_completion
#
#RUN echo 'source /root/secretd_completion' >> ~/.bashrc
#RUN echo 'source /root/secretcli_completion' >> ~/.bashrc

#ENV LD_LIBRARY_PATH=/opt/sgxsdk/libsgx-enclave-common/:/opt/sgxsdk/lib64/

# Run secretd by default, omit entrypoint to ease using container with secretcli
ENTRYPOINT ["/bin/bash", "startup.sh"]
