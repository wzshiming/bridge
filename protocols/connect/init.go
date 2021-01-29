package connect

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("http", bridge.BridgeFunc(CONNECT))
	chain.Default.Register("https", bridge.BridgeFunc(CONNECT))
}
