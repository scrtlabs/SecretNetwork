all:
	$(MAKE) -C contract-v1
	$(MAKE) -C contract-v0.10

run-localsecret:
	docker compose -f ../deployment/dockerfiles/ibc/docker-compose.yml up

kill-localsecret:
	docker compose -f ../deployment/dockerfiles/ibc/docker-compose.yml kill
	docker compose -f ../deployment/dockerfiles/ibc/docker-compose.yml rm -f