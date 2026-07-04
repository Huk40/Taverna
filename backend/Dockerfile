FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/taverna ./cmd/server

FROM alpine:3.20

RUN adduser -D -H appuser

WORKDIR /app

COPY --from=build /out/taverna /app/taverna
COPY migrations ./migrations

ENV PORT=8080
ENV MIGRATIONS_DIR=/app/migrations

EXPOSE 8080

USER appuser

CMD ["/app/taverna"]
