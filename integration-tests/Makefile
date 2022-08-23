all:
	$(MAKE) -C contract-v1
	$(MAKE) -C contract-v0.10

run-localsecret:
	docker run --rm -it -p 9091:9091 --name localsecret ghcr.io/scrtlabs/localsecret:"$(TAG)"