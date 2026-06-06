FROM golang:1.25-bookworm AS builder

# CGO is required: confluent-kafka-go (librdkafka) and chai2010/webp are cgo
# packages. With CGO_ENABLED=0 they compile to empty stubs and the build fails
# with "undefined: kafka.Consumer" / "undefined: webpGetInfo".
RUN apt-get update \
    && apt-get install -y --no-install-recommends gcc libc6-dev ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# confluent-kafka-go ships a self-contained static librdkafka for glibc, so no
# build tags or system librdkafka are needed here.
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /server ./cmd/app

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r appgroup && useradd -r -g appgroup appuser

COPY --from=builder /server /server
COPY --from=builder /app/sql/ /app/sql/

RUN mkdir -p /app/logs /app/storage && chown -R appuser:appgroup /app

USER appuser

WORKDIR /app

EXPOSE 8080

CMD ["/server"]
