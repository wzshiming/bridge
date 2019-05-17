package chain

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/socks5"
	"github.com/wzshiming/bridge/ssh"
)

var Default = bridge.BridgeChain{
	"ssh":    bridge.BridgeFunc(ssh.SSH),
	"socks5": bridge.BridgeFunc(socks5.SOCKS5),
}
