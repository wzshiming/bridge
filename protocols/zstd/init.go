package zstd

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("zstd", bridge.BridgeFunc(ZStd))
}
