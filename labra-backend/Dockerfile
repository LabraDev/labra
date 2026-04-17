FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/labra-api ./cmd

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/labra-api /app/bin/labra-api
COPY --from=builder /src/sql /app/sql

ENV APP_ENV=prod
ENV API_HOST=0.0.0.0
ENV API_PORT=8080
ENV DB_URL=/mnt/labra-db/labra.db

EXPOSE 8080

WORKDIR /app/bin

CMD ["/app/bin/labra-api"]
