FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

RUN mkdir -p /kinitiras/log
WORKDIR /kinitiras

COPY kinitiras-webhook /kinitiras/webhook
LABEL org.opencontainers.image.source=https://github.com/k-cloud-labs/kinitiras

ENTRYPOINT ["/kinitiras/webhook"]