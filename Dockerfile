FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o tupochat .

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/tupochat .

RUN addgroup -g 1000 chatapp && \
    adduser -D -u 1000 -G chatapp chatapp && \
    chown -R chatapp:chatapp /app

USER chatapp

EXPOSE 5522

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD nc -z localhost 5522 || exit 1

CMD ["./tupochat"]
