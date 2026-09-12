FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/worker ./cmd/worker

FROM migrate/migrate:v4.18.3 AS migration

FROM alpine:latest

WORKDIR /

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

COPY --from=builder --chown=app:app /app/server /server
COPY --from=builder --chown=app:app /app/worker /worker
COPY --from=migration --chown=app:app /usr/local/bin/migrate /migrate
COPY --chown=app:app migrations /migrations

USER 10001:10001

EXPOSE 8080

CMD ["/server"]
