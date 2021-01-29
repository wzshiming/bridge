package netcat

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("nc", bridge.BridgeFunc(NetCat))
	chain.Default.Register("netcat", bridge.BridgeFunc(NetCat))
}
