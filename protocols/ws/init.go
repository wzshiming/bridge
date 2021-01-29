package ws

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("ws", bridge.BridgeFunc(WS))
	chain.Default.Register("wss", bridge.BridgeFunc(WS))
}
