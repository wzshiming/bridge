module github.com/wzshiming/bridge

go 1.15

require (
	github.com/spf13/pflag v1.0.5
	github.com/wzshiming/commandproxy v0.1.0
	github.com/wzshiming/httpproxy v0.3.1
	github.com/wzshiming/socks4 v0.1.0
	github.com/wzshiming/socks5 v0.1.0
	golang.org/x/crypto v0.0.0-20200510223506-06a226fb4e37
	golang.org/x/net v0.0.0-20200520182314-0ba52f642ac2
)

replace (
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20200510223506-06a226fb4e37
	golang.org/x/net => golang.org/x/net v0.0.0-20200520182314-0ba52f642ac2
	golang.org/x/sys => golang.org/x/sys v0.0.0-20200523222454-059865788121
)
