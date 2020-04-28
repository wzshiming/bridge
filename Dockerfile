FROM golang:alpine AS builder
WORKDIR /go/src/github.com/wzshiming/bridge/
COPY . .
ENV CGO_ENABLED=0
RUN go install ./cmd/bridge

FROM alpine
COPY --from=builder /go/bin/bridge /usr/local/bin/
ENTRYPOINT [ "/usr/local/bin/bridge" ]
