FROM golang:1.18 AS builder

WORKDIR /go/src/github.com/k-cloud-labs/kinitiras
COPY . /go/src/github.com/k-cloud-labs/kinitiras

RUN make kinitiras-webhook
RUN mv /go/src/github.com/k-cloud-labs/kinitiras/kinitiras-webhook /bin/


FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

RUN mkdir -p /kinitiras/log
WORKDIR /kinitiras

COPY --from=builder /bin/kinitiras-webhook /kinitiras/webhook

ENTRYPOINT ["/kinitiras/webhook"]