FROM golang:1.17 AS builder
WORKDIR /tmp/gamma
COPY ./ /tmp/gamma
RUN go get -v -t -d ./... && \
    go build -o "gateway" cmd/gateway/main.go


FROM alpine:latest
COPY --from=builder /tmp/gamma/gateway /bin/
# 参考: Alpine Linux で go バイナリを起動しようとすると not found が発生する - Qiita
#   URL https://qiita.com/otsuka_kenchan/items/6686a28ac79d54dbe769
RUN apk add --no-cache libc6-compat

ENV env "production"
ENV level "warn"
ENV caller "false"
ENV managerHost "localhost"
ENV managerPort "1883"
ENV gatewayHost "localhost"
ENV gatewayPort "1883"
ENTRYPOINT ["/bin/sh", "-c", "/bin/gateway -env=${env} -level=${level} -caller=${caller} -managerHost=${managerHost} -managerPort=${managerPort} -gatewayHost=${gatewayHost} -gatewayPort=${gatewayPort}"]
