package snappy

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("snappy", bridge.BridgeFunc(Snappy))
}
