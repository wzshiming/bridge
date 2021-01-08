module github.com/wzshiming/bridge

go 1.15

require (
	github.com/spf13/pflag v1.0.5
	github.com/wzshiming/anyproxy v0.2.0
	github.com/wzshiming/cmux v0.1.0
	github.com/wzshiming/commandproxy v0.2.0
	github.com/wzshiming/httpproxy v0.3.2
	github.com/wzshiming/notify v0.0.5
	github.com/wzshiming/shadowsocks v0.1.0
	github.com/wzshiming/socks4 v0.2.2
	github.com/wzshiming/socks5 v0.2.2
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/net v0.0.0-20201224014010-6772e930b67b
)

replace (
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/net => golang.org/x/net v0.0.0-20201224014010-6772e930b67b
	golang.org/x/sys => golang.org/x/sys v0.0.0-20210105210732-16f7687f5001
)
