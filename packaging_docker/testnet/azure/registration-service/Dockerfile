FROM cashmaney/enigma-sgx-base

# wasmi-sgx-test script requirements
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    python3-pip && \
    rm -rf /var/lib/apt/lists/*

COPY enigma_package/secretcli /usr/bin/
# COPY enigma_package/libgo_cosmwasm.so /usr/lib/

COPY requirements.txt .

RUN pip3 install -r requirements.txt

COPY svc.py .

CMD python3 -m svc