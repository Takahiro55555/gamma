FROM golang:1.17 AS builder
WORKDIR /tmp/gamma
COPY ./ /tmp/gamma
RUN ls -la && sleep 5
# RUN go mod init
RUN go get -v -t -d ./...
RUN go build -o "dmb" cmd/dmb/main.go


FROM alpine:latest
COPY --from=builder /tmp/gamma/dmb /bin/

ENV env "production"
ENV level "warn"
ENV caller "false"
ENV managerHost "localhost"
ENV managerPort "1883"
ENV dmbHost "localhost"
ENV dmbPort "1883"
ENV dmbTopic "/"
ENV baseRetransmissionIntervalMilliSeconds "10"
ENV maxRetransmissionIntervalMilliSeconds "5000"
ENTRYPOINT ["/bin/dmb", "-env", "${env}", "-level", "${level}", "-caller", "${caller}", "-managerHost", "${managerHost}", "-managerPort", "${managerPort}", "-dmbHost", "${dmbHost}", "-dmbPort", "${dmbPort}", "-dmbTopic", "${dmbTopic}", "-baseRetransmissionIntervalMilliSeconds", "${baseRetransmissionIntervalMilliSeconds}", "-maxRetransmissionIntervalMilliSeconds", "${maxRetransmissionIntervalMilliSeconds}"]
