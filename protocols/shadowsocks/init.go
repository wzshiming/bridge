package shadowsocks

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("ss", bridge.BridgeFunc(ShadowSocks))
	chain.Default.Register("shadowsocks", bridge.BridgeFunc(ShadowSocks))
}
