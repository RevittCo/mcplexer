.PHONY: build run dev start stop setup test lint clean web-build go-build secret

build: web-build go-build

web-build:
	cd web && npm ci && npm run build

go-build:
	go build -o bin/mcplexer ./cmd/mcplexer

run: build
	-./bin/mcplexer daemon stop 2>/dev/null
	./bin/mcplexer daemon start

dev:
	go run ./cmd/mcplexer serve --mode=http --addr=:3333

start:
	./bin/mcplexer daemon start

stop:
	./bin/mcplexer daemon stop

setup:
	./bin/mcplexer setup

test:
	go test ./...

lint:
	golangci-lint run

secret:
	@if [ -z "$(SCOPE)" ] || [ -z "$(KEY)" ] || [ -z "$(VALUE)" ]; then \
		echo "Usage: make secret SCOPE=<scope-id> KEY=<key> VALUE=<value>"; \
		exit 1; \
	fi
	./bin/mcplexer secret put $(SCOPE) $(KEY) $(VALUE)

clean:
	rm -rf bin/ web/dist/
