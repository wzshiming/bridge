FROM golang:alpine AS builder
WORKDIR /go/src/github.com/wzshiming/bridge/
COPY . .
RUN go install -mod vendor ./cmd/bridge

FROM wzshiming/upx AS upx
COPY --from=builder /go/bin/ /go/bin/
RUN upx /go/bin/*

FROM alpine
COPY --from=upx /go/bin/bridge /usr/local/bin/
ENTRYPOINT [ "bridge" ]
