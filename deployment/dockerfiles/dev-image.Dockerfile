# Final image
FROM build-release

COPY deployment/docker/devimage/bootstrap_init_no_stop.sh bootstrap_init.sh
COPY deployment/docker/devimage/faucet/faucet_server.js .

ENTRYPOINT ["./bootstrap_init.sh"]