package tls

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("tls", bridge.BridgeFunc(TLS))
}
