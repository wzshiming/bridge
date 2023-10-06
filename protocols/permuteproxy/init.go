package permuteproxy

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.RegisterDefault(bridge.BridgeFunc(PermuteProxy))
}
