package socks

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("socks4", bridge.BridgeFunc(SOCKS))
	chain.Default.Register("socks4a", bridge.BridgeFunc(SOCKS))
}
