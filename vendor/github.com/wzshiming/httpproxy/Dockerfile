FROM golang:alpine AS builder
WORKDIR /go/src/github.com/wzshiming/httpproxy/
COPY . .
RUN go install ./cmd/httpproxy

FROM alpine
EXPOSE 8080
COPY --from=builder /go/bin/httpproxy /usr/local/bin/
ENTRYPOINT [ "httpproxy" ]
