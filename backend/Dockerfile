FROM golang:1.24-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    libopencv-dev pkg-config gcc g++ \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o server ./cmd/server

FROM debian:bookworm-slim

# Runtime OpenCV for gocv (videoio pulls FFmpeg as transitive deps on bookworm).
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates sqlite3 \
    libopencv-core406 libopencv-imgproc406 libopencv-imgcodecs406 \
    libopencv-videoio406 libopencv-highgui406 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /build/server .

ENV AI_SERVICE_ADDR=ai_service:50051
ENV SERVER_PORT=8080
ENV DATA_DIR=/app/data

EXPOSE 8080

CMD ["./server"]
