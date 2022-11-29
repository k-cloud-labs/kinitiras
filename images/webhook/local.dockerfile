FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

RUN mkdir -p /kinitiras/log
WORKDIR /kinitiras

COPY kinitiras-webhook /kinitiras/webhook

ENTRYPOINT ["/kinitiras/webhook"]