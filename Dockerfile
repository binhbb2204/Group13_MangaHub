# syntax=docker/dockerfile:1.6
ARG GO_VERSION=1.25-alpine
FROM golang:${GO_VERSION} AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY . .
ARG SERVICE
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/${SERVICE}
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
RUN mkdir -p /app/data /app/logs
COPY --from=builder /out/server /app/server
ENV DB_PATH=/app/data/mangahub.db
CMD ["/app/server"]
