package command

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("cmd", bridge.BridgeFunc(COMMAND))
	chain.Default.Register("command", bridge.BridgeFunc(COMMAND))
}
