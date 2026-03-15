.SILENT:
.PHONY: build run up down logs restart

GO := $(shell which go 2>/dev/null || echo /usr/local/go/bin/go)

build:
	$(GO) build -o ./.bin/main ./...

run: build
	./.bin/main

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f app

restart:
	docker compose restart app
