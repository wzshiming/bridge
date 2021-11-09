package emux

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("emux", bridge.BridgeFunc(EMux))
}
