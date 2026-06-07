FROM golang:1.25-bookworm AS builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends gcc libc6-dev ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

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
