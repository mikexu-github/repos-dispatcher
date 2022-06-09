FROM alpine as certs
RUN apk update && apk add ca-certificates

FROM golang:1.16.0-alpine3.13 AS builder

WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 go build -o dispatcher -mod=vendor -ldflags='-s -w'  -installsuffix cgo cmd/main.go

FROM scratch
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

WORKDIR /dispatcher
COPY --from=builder ./build/dispatcher ./cmd/

EXPOSE 80

ENTRYPOINT ["./cmd/dispatcher","-config=/configs/config.yml"]
