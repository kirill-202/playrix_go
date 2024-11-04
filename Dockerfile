FROM golang:1.23.2 AS builder

WORKDIR /app

COPY . .

RUN go build -o int_go .

FROM alpine:latest

COPY --from=builder /app/int_go /usr/local/bin/int_go

CMD ["int_go"]
