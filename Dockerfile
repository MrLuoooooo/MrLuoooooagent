# ===== Build stage =====
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/goagent ./cmd/server

# ===== Runtime stage =====
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata bash curl

WORKDIR /app

COPY --from=builder /build/goagent ./goagent
COPY --from=builder /build/configs/config.docker.yaml ./configs/config.docker.yaml

RUN mkdir -p /var/log/goagent

EXPOSE 8080

ENV CONFIG_PATH=/app/configs
ENV CONFIG_NAME=config.docker

ENTRYPOINT ["./goagent"]
