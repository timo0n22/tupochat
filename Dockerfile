FROM golang:1.24-alpine AS builder

WORKDIR /tupochat

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o tupochat

FROM alpine:3.16

WORKDIR /tupochat

COPY --from=builder /tupochat .

CMD ["./tupochat"]