package chain

import (
	"errors"
	"net/url"

	"github.com/wzshiming/bridge"
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
func (b *BridgeChain) BridgeChain(dialer bridge.Dialer, addresses ...string) (bridge.Dialer, bridge.ListenConfig, error) {
	if len(addresses) == 0 {
		return dialer, nil, nil
	}
	address := addresses[len(addresses)-1]
	d, l, err := b.bridge(dialer, address)
	if err != nil {
		return nil, nil, err
	}
	addresses = addresses[:len(addresses)-1]
	if len(addresses) == 0 {
		return d, l, nil
	}
	return b.BridgeChain(d, addresses...)
}

func (b *BridgeChain) bridge(dialer bridge.Dialer, address string) (bridge.Dialer, bridge.ListenConfig, error) {
	ur, err := url.Parse(address)
	if err != nil {
		return nil, nil, err
	}
	dial, ok := b.proto[ur.Scheme]
	if !ok {
		return nil, nil, errors.New("not define " + address)
	}
	return dial.Bridge(dialer, address)
}

// Register is register a new bridger for BridgeChain.
func (b *BridgeChain) Register(name string, bridger bridge.Bridger) error {
	b.proto[name] = bridger
	return nil
}
