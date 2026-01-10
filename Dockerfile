FROM golang:1.24.6-alpine AS builder

RUN addgroup -S appgroup && adduser -S appuser -G appgroup \
    && apk add --no-cache ca-certificates tzdata upx

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server ./cmd/app

RUN upx --best --lzma /server

RUN mkdir -p /app/logs && chown appuser:appgroup /app/logs

RUN mkdir -p /tmp && chown appuser:appgroup /tmp

FROM alpine:latest

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

COPY --from=builder --chown=appuser:appgroup /server /server
COPY --from=builder /app/config.yaml /config.yaml

COPY --from=builder --chown=appuser:appgroup /app/sql/ /app/sql/

RUN mkdir -p /app/logs && chown appuser:appgroup /app/logs

USER appuser

WORKDIR /

EXPOSE 8080

CMD ["/server"]