module github.com/wzshiming/bridge

go 1.13

require (
	github.com/spf13/pflag v1.0.5
	github.com/wzshiming/httpproxy v0.1.0
	github.com/wzshiming/socks v0.0.0-20191031031631-473648b72a10
	golang.org/x/crypto v0.0.0-20190308221718-c2843e01d9a2
	golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3
)

replace (
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20200208060501-ecb85df21340
	golang.org/x/net => golang.org/x/net v0.0.0-20200202094626-16171245cfb2
	golang.org/x/sys => golang.org/x/sys v0.0.0-20200202164722-d101bd2416d5
)
