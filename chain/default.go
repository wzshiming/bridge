package chain

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/connect"
	"github.com/wzshiming/bridge/socks"
	"github.com/wzshiming/bridge/socks5"
	"github.com/wzshiming/bridge/ssh"
)

var Default = BridgeChain{
	"ssh":     bridge.BridgeFunc(ssh.SSH),
	"socks4":  bridge.BridgeFunc(socks.SOCKS),
	"socks4a": bridge.BridgeFunc(socks.SOCKS),
	"socks5":  bridge.BridgeFunc(socks5.SOCKS5),
	"socks5h": bridge.BridgeFunc(socks5.SOCKS5),
	"http":    bridge.BridgeFunc(connect.CONNECT),
	"https":   bridge.BridgeFunc(connect.CONNECT),
}
