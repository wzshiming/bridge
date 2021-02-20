package chain

import (
	"fmt"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/bridge/multiple/lb"
)

// BridgeChain is a bridger that supports multiple crossing of bridger.
type BridgeChain struct {
	proto map[string]bridge.Bridger
}

// NewBridgeChain create a new BridgeChain.
func NewBridgeChain() *BridgeChain {
	return &BridgeChain{map[string]bridge.Bridger{}}
}

// BridgeChain is multiple crossing of bridge.
func (b *BridgeChain) BridgeChain(dialer bridge.Dialer, addresses ...string) (bridge.Dialer, error) {
	if len(addresses) == 0 {
		return dialer, nil
	}
	address := addresses[len(addresses)-1]
	d, err := b.bridge(dialer, address)
	if err != nil {
		return nil, err
	}
	addresses = addresses[:len(addresses)-1]
	if len(addresses) == 0 {
		return d, nil
	}
	return b.BridgeChain(d, addresses...)
}

func (b *BridgeChain) bridge(dialer bridge.Dialer, address string) (bridge.Dialer, error) {
	addresses := strings.Split(address, "|")
	dialers := make([]bridge.Dialer, 0, len(addresses))
	for _, address := range addresses {
		sch, _, ok := scheme.SplitSchemeAddr(address)
		if !ok {
			return nil, fmt.Errorf("unsupported protocol format %q", address)
		}
		bridger, ok := b.proto[sch]
		if !ok {
			return nil, fmt.Errorf("unsupported protocol %q", sch)
		}

		dial, err := bridger.Bridge(dialer, address)
		if err != nil {
			return nil, err
		}

		dialers = append(dialers, dial)
	}
	return lb.NewDialer(dialers), nil
}

// Register is register a new bridger for BridgeChain.
func (b *BridgeChain) Register(name string, bridger bridge.Bridger) error {
	b.proto[name] = bridger
	return nil
}
