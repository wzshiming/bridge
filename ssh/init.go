package ssh

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

func init() {
	chain.Default.Register("ssh", bridge.BridgeFunc(SSH))
}
