ENV_PATH ?= .env

-include $(ENV_PATH)

export

COMPOSE_PATH ?= docker-compose.yml
DOCKER_COMPOSE := docker compose -f $(COMPOSE_PATH) --env-file $(ENV_PATH)

DB_HOST_FOR_MIGRATE ?= localhost
DB_PORT_FOR_MIGRATE ?= 5433
DB_URL := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST_FOR_MIGRATE):$(DB_PORT_FOR_MIGRATE)/$(DB_NAME)?sslmode=disable

MIGRATIONS_PATH := migrations
COVERAGE_FILE := coverage.out
GO_PACKAGES := $(shell go list ./... | grep -v '/mocks')

.PHONY: generate test coverage-html clean docker-build docker-up docker-down docker-stop docker-logs migrate-up migrate-down

generate:
	@echo "==> Generating..."
	@go generate ./...

test:
	@echo "==> Running tests with coverage..."
	@go test -covermode=atomic -coverprofile=$(COVERAGE_FILE) $(GO_PACKAGES)
	@echo
	@echo "==> Total coverage:"
	@go tool cover -func=$(COVERAGE_FILE) | grep total

coverage-html: test
	@echo "==> Opening HTML coverage report..."
	@go tool cover -html=$(COVERAGE_FILE)

clean:
	@echo "==> Cleaning generated files..."
	@rm -f $(COVERAGE_FILE)

docker-build:
	@echo "==> Building Docker images..."
	@$(DOCKER_COMPOSE) build

docker-up:
	@echo "==> Starting Docker Compose in detached mode..."
	@$(DOCKER_COMPOSE) up -d

docker-down:
	@echo "==> Stopping and removing containers..."
	@$(DOCKER_COMPOSE) down

docker-stop:
	@echo "==> Stopping Docker Compose services..."
	@$(DOCKER_COMPOSE) stop

docker-logs:
	@echo "==> Following container logs..."
	@$(DOCKER_COMPOSE) logs -f

migrate-up:
	@echo "==> Applying migrations..."
	@migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" up

migrate-down:
	@echo "==> Rolling back migrations..."
	@migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" down
