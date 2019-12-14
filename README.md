# Bridge

Bridge is a TCP proxy tool Support http(s)-connect socks4/4a/5/5h ssh

[![Build Status](https://travis-ci.org/wzshiming/bridge.svg?branch=master)](https://travis-ci.org/wzshiming/bridge)
[![Go Report Card](https://goreportcard.com/badge/github.com/wzshiming/bridge)](https://goreportcard.com/report/github.com/wzshiming/bridge)
[![GoDoc](https://godoc.org/github.com/wzshiming/bridge?status.svg)](https://godoc.org/github.com/wzshiming/bridge)
[![Docker Automated build](https://img.shields.io/docker/cloud/automated/wzshiming/bridge.svg)](https://hub.docker.com/r/wzshiming/bridge)
[![GitHub license](https://img.shields.io/github/license/wzshiming/bridge.svg)](https://github.com/wzshiming/bridge/blob/master/LICENSE)

- [English](https://github.com/wzshiming/bridge/blob/master/README.md)
- [简体中文](https://github.com/wzshiming/bridge/blob/master/README_cn.md)

## Example

Mapping github.io: 80 TCP port to 8080 port of the local machine  
access using IP will return 404 pages.  

``` shell
bridge -b :8080 -p github.io:80
```

Proxy that can go through various protocols  

``` shell
bridge -b :8080 -p github.io:80 -p ssh://username:password@my_server:22
bridge -b :8080 -p github.io:80 -p ssh://username@my_server:22?identity_file=~/.ssh/id_rsa
bridge -b :8080 -p github.io:80 -p socks5://username:password@my_server:1080
bridge -b :8080 -p github.io:80 -p http://username:password@my_server:8080
```

It can also go through multi-level proxy  

``` shell
bridge -b :8080 -p github.io:80 -p http://username:password@my_server2:8080 -p http://username:password@my_server1:8080

```

You can also use ssh to listen for port mapping from local port to server port,  
due to the limitation of sshd, only 127.0.0.1 ports can be monitored.  
if you want to provide external services,  
you need to change the gatewayports no in /etc/ssh/ sshd_config to yes  
and then reload sshd.  

``` shell
bridge -b :8080 -b ssh://username:password@my_server:22 -p 127.0.0.1:80
```

More of the time I'm acting as an ssh proxy  
in ~/.ssh/config  

``` text
ProxyCommand bridge -p %h:%p -p "ssh://username@my_server?identity_file=~/.ssh/id_rsa"
```

## Usage

``` text
        bridge [-d] \
        [-b=[bind_address]:bind_port \
        [-b=ssh://bridge_bind_address:bridge_bind_port [-b=(socks5|socks4|socks4a|https|http|ssh)://bridge_bind_address:bridge_bind_port ...]]] \ //
        -p=proxy_address:proxy_port \
        [-p=(socks4|socks4a|socks5|socks5h|https|http|ssh)://bridge_proxy_address:bridge_proxy_port ...]
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

[Docker image](https://hub.docker.com/r/wzshiming/bridge)

## License

Pouch is licensed under the MIT License. See [LICENSE](https://github.com/wzshiming/bridge/blob/master/LICENSE) for the full license text.
