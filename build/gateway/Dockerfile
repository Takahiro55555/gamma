FROM golang:1.17 AS builder
WORKDIR /tmp/gamma
COPY ./ /tmp/gamma
RUN go get -v -t -d ./... && \
    go build -o "gateway" cmd/gateway/main.go


FROM alpine:latest
COPY --from=builder /tmp/gamma/gateway /bin/

ENV env "production"
ENV level "warn"
ENV caller "false"
ENV managerHost "localhost"
ENV managerPort "1883"
ENV gatewayHost "localhost"
ENV gatewayPort "1884"
ENTRYPOINT ["/bin/gateway","-env", "${env}", "-level", "${level}", "-caller", "${caller}", "-managerHost", "${managerHost}", "-managerPort", "${managerPort}", "-gatewayHost", "${gatewayHost}", "-gatewayPort", "${gatewayPort}"]
