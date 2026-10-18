MONGO_CONTAINER ?= requirement-pipeline-mongo
MONGO_IMAGE ?= mongo:7
MONGO_PORT ?= 27017
MONGO_VOLUME ?= requirement-pipeline-mongo-data
WEB_ADDR ?= 127.0.0.1:8080

.PHONY: web mongo mongo-stop

web: mongo
	go run ./cmd/requirement-pipeline -web -addr $(WEB_ADDR)

mongo:
	@if docker container inspect "$(MONGO_CONTAINER)" >/dev/null 2>&1; then \
		if [ "$$(docker inspect --format '{{.State.Running}}' "$(MONGO_CONTAINER)")" != "true" ]; then \
			docker start "$(MONGO_CONTAINER)" >/dev/null; \
		fi; \
	else \
		docker run --detach \
			--name "$(MONGO_CONTAINER)" \
			--publish "$(MONGO_PORT):27017" \
			--volume "$(MONGO_VOLUME):/data/db" \
			"$(MONGO_IMAGE)" >/dev/null; \
	fi
	@attempt=0; \
	until docker exec "$(MONGO_CONTAINER)" mongosh --quiet --eval 'db.adminCommand({ ping: 1 }).ok' >/dev/null 2>&1; do \
		attempt=$$((attempt + 1)); \
		if [ "$$attempt" -ge 30 ]; then \
			printf '%s\n' 'MongoDB did not become ready within 30 seconds.' >&2; \
			exit 1; \
		fi; \
		sleep 1; \
	done

mongo-stop:
	@if [ "$$(docker inspect --format '{{.State.Running}}' "$(MONGO_CONTAINER)" 2>/dev/null)" = "true" ]; then \
		docker stop "$(MONGO_CONTAINER)" >/dev/null; \
	fi
