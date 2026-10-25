FROM golang:1.26.10-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/taverna ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/taverna-admin ./cmd/admin

FROM alpine:3.23

RUN adduser -D -H appuser

WORKDIR /app

COPY --from=build /out/taverna /app/taverna
COPY --from=build /out/taverna-admin /app/taverna-admin
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY migrations ./migrations
RUN mkdir -p /app/storage/model-assets /app/storage/export-artifacts && chown -R appuser:appuser /app/storage

ENV PORT=8080
ENV MIGRATIONS_DIR=/app/migrations

EXPOSE 8080

USER appuser
ENTRYPOINT ["/app/taverna"]
