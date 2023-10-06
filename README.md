# Bridge

Bridge is a TCP proxy tool Support http(s)-connect socks4/4a/5/5h ssh proxycommand

[![Build](https://github.com/wzshiming/bridge/actions/workflows/go-cross-build.yml/badge.svg)](https://github.com/wzshiming/bridge/actions/workflows/go-cross-build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/wzshiming/bridge)](https://goreportcard.com/report/github.com/wzshiming/bridge)
[![GoDoc](https://godoc.org/github.com/wzshiming/bridge?status.svg)](https://godoc.org/github.com/wzshiming/bridge)
[![Docker Automated build](https://img.shields.io/docker/cloud/automated/wzshiming/bridge.svg)](https://hub.docker.com/r/wzshiming/bridge)
[![GitHub license](https://img.shields.io/github/license/wzshiming/bridge.svg)](https://github.com/wzshiming/bridge/blob/master/LICENSE)

- [English](https://github.com/wzshiming/bridge/blob/master/README.md)
- [简体中文](https://github.com/wzshiming/bridge/blob/master/README_cn.md)

## Supported protocols

- [Socks4](https://github.com/wzshiming/socks4)
- [Socks5](https://github.com/wzshiming/socks5)
- [HTTP Proxy](https://github.com/wzshiming/httpproxy)
- [Shadow Socks](https://github.com/wzshiming/shadowsocks)
- [SSH Proxy](https://github.com/wzshiming/sshproxy)
- [Any Proxy](https://github.com/wzshiming/anyproxy)
- [Emux](https://github.com/wzshiming/emux)

## Example

Mapping example.org:80 TCP port to 8080 port of the local machines.  

``` shell
bridge -b :8080 -p example.org:80
# `curl -H 'Host: example.org' 127.0.0.1:8080` will return to the target page
```

Proxy that can go through various protocols.  

``` shell
bridge -b :8080 -p example.org:80 -p ssh://username:password@my_server:22
bridge -b :8080 -p example.org:80 -p ssh://username@my_server:22?identity_file=~/.ssh/id_rsa
bridge -b :8080 -p example.org:80 -p socks5://username:password@my_server:1080
bridge -b :8080 -p example.org:80 -p http://username:password@my_server:8080
bridge -b :8080 -p example.org:80 -p 'cmd:nc %h %p'
bridge -b :8080 -p example.org:80 -p 'cmd:ssh sshserver nc %h %p'
```

It can also go through multi-level proxy.  

``` shell
bridge -b :8080 -p example.org:80 -p http://username:password@my_server2:8080 -p http://username:password@my_server1:8080
```

Using proxy protocol(http/socks4/socks5) instead of direct TCP forwarding.  

``` shell
bridge -b :8080 -p -
bridge -b :8080 -p - -p http://username:password@my_server1:8080
# `http_proxy=http://127.0.0.1:8080 curl example.org` Will be the proxy
```

You can also use ssh to listen for port mapping from local port to server port,  
due to the limitation of sshd, only 127.0.0.1 ports can be monitored.  
if you want to provide external services,  
you need to change the 'GatewayPorts no' in /etc/ssh/sshd_config to yes  
and then reload sshd.  

``` shell
bridge -b :8080 -b ssh://username:password@my_server:22 -p 127.0.0.1:80
```

More of the time I'm acting as a ssh proxy.  

``` text
# in ~/.ssh/config
ProxyCommand bridge -p %h:%p -p "ssh://username@my_server?identity_file=~/.ssh/id_rsa"
```

## Usage

``` text
Usage: bridge [-d] \
	[-b=[[tcp://]bind_address]:bind_port \
	[-b=ssh://bridge_bind_address:bridge_bind_port [-b=(socks4://|socks4a://|socks5://|socks5h://|https://|http://|ssh://|cmd:)bridge_bind_address:bridge_bind_port ...]]] \ // 
	-p=([tcp://]proxy_address:proxy_port|-) \
	[-p=(socks4://|socks4a://|socks5://|socks5h://|https://|http://|ssh://|cmd:)bridge_proxy_address:bridge_proxy_port ...]
  -b, --bind strings    The first is the listening address, and then the proxy through which the listening address passes.
                        If it is not filled in, it is redirected to the pipeline.
                        only SSH and local support listening, so the last proxy must be ssh.
  -d, --debug           Output the communication data.
  -p, --proxy strings   The first is the dial-up address, followed by the proxy through which the dial-up address passes.
```

## Install

``` shell
go get -u -v github.com/wzshiming/bridge/cmd/bridge
```

or

[Download releases](https://github.com/wzshiming/bridge/releases)

or

[Image](https://github.com/wzshiming/bridge/pkgs/container/bridge%2Fbridge)

## License

Licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/bridge/blob/master/LICENSE) for the full license text.
