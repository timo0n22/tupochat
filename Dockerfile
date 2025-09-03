FROM golang:1.19.0-alpine3.16 AS builder

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