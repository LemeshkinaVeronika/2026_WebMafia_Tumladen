APP_NAME=server

.PHONY: tidy fmt test build run compose-up compose-down

tidy:
	go mod tidy

fmt:
	gofmt -w cmd internal pkg

test:
	go test ./...

build:
	go build -o bin/$(APP_NAME) ./cmd/server

run:
	go run ./cmd/server

compose-up:
	docker compose up --build

compose-down:
	docker compose down -v
