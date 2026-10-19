.PHONY: test build run up down lint-api
test:
	cd api && go vet ./... && go test ./...
build:
	cd api && go build -o ../bin/api ./cmd/api
run:
	cd api && go run ./cmd/api
up:
	docker compose -f deploy/docker-compose.yml up -d
down:
	docker compose -f deploy/docker-compose.yml down
