FROM golang:1.23.2-alpine AS builder 

WORKDIR /app

COPY . .

RUN go build -o int_go .

FROM alpine:latest

RUN apk add --no-cache libc6-compat && mkdir /app_config

COPY --from=builder /app/int_go /bin/int_go

RUN chmod +x /bin/int_go

CMD ["/bin/int_go"]


