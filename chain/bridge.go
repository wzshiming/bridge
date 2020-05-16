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
	ur, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	dial, ok := b.proto[ur.Scheme]
	if !ok {
		return nil, errors.New("not define " + address)
	}
	return dial.Bridge(dialer, address)
}

// Register is register a new bridger for BridgeChain.
func (b *BridgeChain) Register(name string, bridger bridge.Bridger) error {
	b.proto[name] = bridger
	return nil
}
