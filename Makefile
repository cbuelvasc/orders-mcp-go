.PHONY: up down run test integration lint build

up:            ## start Postgres, Keycloak, Prometheus and Grafana
	docker compose up -d

down:
	docker compose down

run:           ## run the MCP server on :8080
	go run ./cmd/server

test:          ## unit + contract (in-memory) + architecture tests
	go test ./...

integration:   ## contract tests against real PostgreSQL (Testcontainers, needs Docker)
	go test -tags=integration ./...

lint:
	go vet ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/orders-mcp ./cmd/server
