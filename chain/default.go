package chain

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/connect"
	"github.com/wzshiming/bridge/socks"
	"github.com/wzshiming/bridge/ssh"
)

var Default = BridgeChain{
	"ssh":     bridge.BridgeFunc(ssh.SSH),
	"socks5":  bridge.BridgeFunc(socks.SOCKS),
	"socks4":  bridge.BridgeFunc(socks.SOCKS),
	"socks4a": bridge.BridgeFunc(socks.SOCKS),
	"http":    bridge.BridgeFunc(connect.CONNECT),
	"https":   bridge.BridgeFunc(connect.CONNECT),
}
