# Bridge

Bridge 是一个支持 http(s)-connect socks4/4a/5/5h ssh 的tcp代理工具

[![Build Status](https://travis-ci.org/wzshiming/bridge.svg?branch=master)](https://travis-ci.org/wzshiming/bridge)
[![Go Report Card](https://goreportcard.com/badge/github.com/wzshiming/bridge)](https://goreportcard.com/report/github.com/wzshiming/bridge)
[![GoDoc](https://godoc.org/github.com/wzshiming/bridge?status.svg)](https://godoc.org/github.com/wzshiming/bridge)
[![GitHub license](https://img.shields.io/github/license/wzshiming/bridge.svg)](https://github.com/wzshiming/bridge/blob/master/LICENSE)

- [English](https://github.com/wzshiming/bridge/blob/master/README.md)
- [简体中文](https://github.com/wzshiming/bridge/blob/master/README_cn.md)

## 示例

映射 github.io:80 tcp 端口到本机的 8080 端口  
由于是使用 ip 访问的 访问会返回 404 页面  

``` shell
bridge -b :8080 -p github.io:80
```

可以经过各种协议的代理  

``` shell
bridge -b :8080 -p github.io:80 -p ssh://username:password@my_server:22
bridge -b :8080 -p github.io:80 -p ssh://username@my_server:22?identity_file=~/.ssh/id_rsa
bridge -b :8080 -p github.io:80 -p socks5://username:password@my_server:1080
bridge -b :8080 -p github.io:80 -p http://username:password@my_server:8080
```

也可以经过多级代理  

``` shell
bridge -b :8080 -p github.io:80 -p http://username:password@my_server2:8080 -p http://username:password@my_server1:8080

```

也可以通过 ssh 监听端口 本地的端口映射到服务器的端口  
由于 sshd 的限制只能监听 127.0.0.1 的端口  
如果想提供对外的服务需要把 /etc/ssh/sshd_config 里的 GatewayPorts no 改成 yes 然后重新加载 sshd  

``` shell
bridge -b :8080 -b ssh://username:password@my_server:22 -p 127.0.0.1:80
```

更多的时候我是用作 ssh 代理的  
在 ~/.ssh/config  

``` text
ProxyCommand bridge -p %h:%p -p "ssh://username@my_server?identity_file=~/.ssh/id_rsa"
```

## 用法

``` text
        bridge [-d] \
        [-b=[bind_address]:bind_port \
        [-b=ssh://bridge_bind_address:bridge_bind_port [-b=(socks5|socks4|socks4a|https|http|ssh)://bridge_bind_address:bridge_bind_port ...]]] \ //
        -p=proxy_address:proxy_port \
        [-p=(socks4|socks4a|socks5|socks5h|https|http|ssh)://bridge_proxy_address:bridge_proxy_port ...]
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

[Docker image](https://hub.docker.com/r/wzshiming/bridge)

## 许可证

软包根据MIT License。有关完整的许可证文本，请参阅[LICENSE](https://github.com/wzshiming/bridge/blob/master/LICENSE)。  