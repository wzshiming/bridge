package socks4

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("socks4", bridge.BridgeFunc(SOCKS4))
	chain.Default.Register("socks4a", bridge.BridgeFunc(SOCKS4))
}
