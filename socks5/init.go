package socks5

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("socks5", bridge.BridgeFunc(SOCKS5))
	chain.Default.Register("socks5h", bridge.BridgeFunc(SOCKS5))
}
