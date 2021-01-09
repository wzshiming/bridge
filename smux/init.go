package smux

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("smux", bridge.BridgeFunc(SMux))
}
