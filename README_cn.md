# Bridge

Bridge 是一个支持 http(s)-connect socks4/4a/5/5h ssh proxycommand 的tcp代理工具

[![Build](https://github.com/wzshiming/bridge/actions/workflows/go-cross-build.yml/badge.svg)](https://github.com/wzshiming/bridge/actions/workflows/go-cross-build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/wzshiming/bridge)](https://goreportcard.com/report/github.com/wzshiming/bridge)
[![GoDoc](https://godoc.org/github.com/wzshiming/bridge?status.svg)](https://godoc.org/github.com/wzshiming/bridge)
[![Docker Automated build](https://img.shields.io/docker/cloud/automated/wzshiming/bridge.svg)](https://hub.docker.com/r/wzshiming/bridge)
[![GitHub license](https://img.shields.io/github/license/wzshiming/bridge.svg)](https://github.com/wzshiming/bridge/blob/master/LICENSE)

- [English](https://github.com/wzshiming/bridge/blob/master/README.md)
- [简体中文](https://github.com/wzshiming/bridge/blob/master/README_cn.md)

## 支持的协议

- [Socks4](https://github.com/wzshiming/socks4)
- [Socks5](https://github.com/wzshiming/socks5)
- [HTTP Proxy](https://github.com/wzshiming/httpproxy)
- [Shadow Socks](https://github.com/wzshiming/shadowsocks)
- [SSH Proxy](https://github.com/wzshiming/sshproxy)
- [Any Proxy](https://github.com/wzshiming/anyproxy)
- [Emux](https://github.com/wzshiming/emux)

## 示例

映射 example.org:80 tcp 端口到本机的 8080 端口.  

``` shell
bridge -b :8080 -p example.org:80
# `curl -H 'Host: example.org' 127.0.0.1:8080` 将返回目标的页面
```

可以经过各种协议的代理.  

``` shell
bridge -b :8080 -p example.org:80 -p ssh://username:password@my_server:22
bridge -b :8080 -p example.org:80 -p ssh://username@my_server:22?identity_file=~/.ssh/id_rsa
bridge -b :8080 -p example.org:80 -p socks5://username:password@my_server:1080
bridge -b :8080 -p example.org:80 -p http://username:password@my_server:8080
bridge -b :8080 -p example.org:80 -p 'cmd:nc %h %p'
bridge -b :8080 -p example.org:80 -p 'cmd:ssh sshserver nc %h %p'
```

也可以经过多级代理  

``` shell
bridge -b :8080 -p example.org:80 -p http://username:password@my_server2:8080 -p http://username:password@my_server1:8080
```

使用代理协议(http/socks4/socks5)代替直接TCP转发.  

``` shell
bridge -b :8080 -p -
bridge -b :8080 -p - -p http://username:password@my_server1:8080
# `http_proxy=http://127.0.0.1:8080 curl example.org` 将经过代理
```

也可以通过 ssh 监听端口 本地的端口映射到服务器的端口,  
由于 sshd 的限制只能监听 127.0.0.1 的端口,  
如果想提供对外的服务需要把 /etc/ssh/sshd_config 里的 GatewayPorts no 改成 yes 然后重新加载 sshd.  

``` shell
bridge -b :8080 -b ssh://username:password@my_server:22 -p 127.0.0.1:80
```

更多的时候我是用作 ssh 代理的.  

``` text
# 在 ~/.ssh/config
ProxyCommand bridge -p %h:%p -p "ssh://username@my_server?identity_file=~/.ssh/id_rsa"
```

## 用法

``` text
Usage: bridge [-d] \
	[-b=[[tcp://]bind_address]:bind_port \
	[-b=ssh://bridge_bind_address:bridge_bind_port [-b=(socks4://|socks4a://|socks5://|socks5h://|https://|http://|ssh://|cmd:)bridge_bind_address:bridge_bind_port ...]]] \ // 
	-p=([tcp://]proxy_address:proxy_port|-) \
	[-p=(socks4://|socks4a://|socks5://|socks5h://|https://|http://|ssh://|cmd:)bridge_proxy_address:bridge_proxy_port ...]
  -b, --bind strings    第一个是侦听地址，然后是侦听地址通过的代理。
                        如果未填写，则重定向到管道。
                        只有ssh和本地支持监听，所以最后一个代理必须是ssh。
  -d, --debug           输出通信数据。
  -p, --proxy strings   第一个是拨号地址，然后是拨号地址通过的代理。
```

## 安装

``` shell
go get -u -v github.com/wzshiming/bridge/cmd/bridge
```

or

[Download releases](https://github.com/wzshiming/bridge/releases)

or

[Docker image](https://hub.docker.com/r/wzshiming/bridge)

## 许可证

软包根据MIT License。有关完整的许可证文本，请参阅[LICENSE](https://github.com/wzshiming/bridge/blob/master/LICENSE)。  
