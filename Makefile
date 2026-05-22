SHELL := /usr/bin/env bash
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/tasks?sslmode=disable
MIGRATIONS   := db/migrations

.PHONY: run test test-race lint sqlc migrate-up migrate-down swag build docker docker-down clean

## run: start the API locally (expects Postgres on $DATABASE_URL)
run:
	go run ./cmd/api

## test: run all unit tests
test:
	go test -count=1 ./...

## test-race: run unit tests with the race detector (requires CGO)
test-race:
	CGO_ENABLED=1 go test -race -count=1 ./...

## lint: run go vet and golangci-lint
lint:
	go vet ./...
	golangci-lint run

## sqlc: regenerate sqlc code from db/queries
sqlc:
	sqlc generate

## migrate-up: apply all pending migrations
migrate-up:
	migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)" up

## migrate-down: revert the last migration
migrate-down:
	migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)" down 1

## swag: regenerate OpenAPI/Swagger docs
swag:
	swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal

## build: build the api binary
build:
	go build -trimpath -o bin/api ./cmd/api

## docker: build and start the full stack (postgres + migrate + api)
docker:
	docker compose up --build

## docker-down: tear it all down (and wipe the volume)
docker-down:
	docker compose down -v

## clean: remove build artifacts
clean:
	rm -rf bin coverage.out cover.out
