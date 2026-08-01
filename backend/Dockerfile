FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/taverna ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache su-exec
RUN adduser -D -H appuser

WORKDIR /app

COPY --from=build /out/taverna /app/taverna
COPY migrations ./migrations
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN mkdir -p /app/storage/model-assets && chown -R appuser:appuser /app/storage
RUN chmod +x /app/docker-entrypoint.sh

ENV PORT=8080
ENV MIGRATIONS_DIR=/app/migrations

EXPOSE 8080

ENTRYPOINT ["/app/docker-entrypoint.sh"]
