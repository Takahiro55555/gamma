FROM golang:1.17 AS builder
WORKDIR /tmp/gamma
COPY ./ /tmp/gamma
RUN go get -v -t -d ./... && \
    go build -o "manager" cmd/manager/main.go

FROM alpine:latest
COPY --from=builder /tmp/gamma/manager /bin/

ENV env "production"
ENV level "warn"
ENV caller "false"
ENV host "127.0.0.1"
ENV port "1883"
ENTRYPOINT ["/bin/manager", "-env", "${env}", "-level", "${level}", "-caller", "${caller}", "-host", "${host}", "-port", "${port}"]
