.PHONY: build run test test-unit lint lint-fix migrate-up migrate-down docker-build docker-up docker-down

include .env
export $(shell sed 's/=.*//' .env)

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -v -count=1

test-race:
	go test -race ./... -v -count=1

test-unit:
	go test ./tests/unit/... -v -count=1

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

docker-build:
	docker build -t satvos:latest .

docker-up:
	docker compose up -d

docker-down:
	docker compose down -v