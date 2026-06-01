FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /build/goagent ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata bash curl

WORKDIR /app

COPY --from=builder /build/goagent ./goagent
COPY --from=builder /build/configs/config.docker.yaml ./configs/config.docker.yaml

RUN mkdir -p /var/log/goagent /app/data/checkpoints

EXPOSE 8080

ENV CONFIG_PATH=/app/configs
ENV CONFIG_NAME=config.docker

ENTRYPOINT ["./goagent"]
