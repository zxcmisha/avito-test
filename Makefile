SHELL := /bin/sh

DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/booking?sslmode=disable
GOMODCACHE ?= $(CURDIR)/.gocache
GOPATH ?= $(CURDIR)/.gopath

.PHONY: up seed swagger loadtest

up:
	docker compose up --build -d

seed:
	DATABASE_URL="$(DATABASE_URL)" GOMODCACHE="$(GOMODCACHE)" GOPATH="$(GOPATH)" go run ./cmd/seed

swagger:
	GOMODCACHE="$(GOMODCACHE)" GOPATH="$(GOPATH)" go run github.com/swaggo/swag/cmd/swag@v1.16.4 init -g cmd/server/main.go -o docs --parseInternal

loadtest:
	GOMODCACHE="$(GOMODCACHE)" GOPATH="$(GOPATH)" go test -run '^$$' -bench BenchmarkSlotsListParallel -benchmem -benchtime=20s ./internal/app | tee loadtest/benchmark.txt
