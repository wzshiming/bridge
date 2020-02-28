FROM golang:alpine AS builder
WORKDIR /go/src/github.com/wzshiming/bridge/
COPY . .
RUN go install ./cmd/bridge

FROM alpine
COPY --from=builder /go/bin/bridge /usr/local/bin/
ENTRYPOINT [ "bridge" ]
