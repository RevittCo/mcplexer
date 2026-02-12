export DOCKER_HOST ?= unix:///var/run/docker.sock

.PHONY: build run stop logs dev test lint clean web-build go-build secret

build: web-build go-build

run:
	mkdir -p data
	cp mcplexer.yaml data/mcplexer.yaml
	docker compose up --build -d
	@echo "MCPlexer running at http://localhost:3333"
	@echo "SQLite DB stored at ./data/mcplexer.db"

stop:
	docker compose down

logs:
	docker compose logs -f

web-build:
	cd web && npm ci && npm run build

go-build:
	go build -o bin/mcplexer ./cmd/mcplexer

dev:
	go run ./cmd/mcplexer serve --mode=http --addr=:3333

test:
	go test ./...

lint:
	golangci-lint run

secret:
	@if [ -z "$(SCOPE)" ] || [ -z "$(KEY)" ] || [ -z "$(VALUE)" ]; then \
		echo "Usage: make secret SCOPE=<scope-id> KEY=<key> VALUE=<value>"; \
		exit 1; \
	fi
	docker compose exec mcplexer mcplexer secret put $(SCOPE) $(KEY) $(VALUE)

clean:
	rm -rf bin/ web/dist/ data/
