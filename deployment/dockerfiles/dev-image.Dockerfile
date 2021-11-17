# Final image
FROM build-release

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    python3.8 \
    python3-pip

COPY deployment/docker/devimage/faucet/requirements.txt .

RUN pip install -r requirements.txt

COPY deployment/docker/devimage/bootstrap_init_no_stop.sh bootstrap_init.sh
COPY deployment/docker/devimage/faucet/svc.py .

ENTRYPOINT ["./bootstrap_init.sh"]