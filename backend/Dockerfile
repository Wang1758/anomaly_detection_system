FROM golang:1.24-bookworm AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o server ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates sqlite3 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /build/server .

ENV AI_SERVICE_ADDR=ai_service:50051
ENV SERVER_PORT=8080
ENV DATA_DIR=/app/data

EXPOSE 8080

CMD ["./server"]
