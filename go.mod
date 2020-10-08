module github.com/wzshiming/bridge

go 1.15

require (
	github.com/spf13/pflag v1.0.5
	github.com/wzshiming/commandproxy v0.1.0
	github.com/wzshiming/httpproxy v0.3.1
	github.com/wzshiming/socks4 v0.2.1
	github.com/wzshiming/socks5 v0.2.1
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/net v0.0.0-20201006153459-a7d1128ccaa0
)

replace (
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/net => golang.org/x/net v0.0.0-20201006153459-a7d1128ccaa0
	golang.org/x/sys => golang.org/x/sys v0.0.0-20201007082116-8445cc04cbdf
)
