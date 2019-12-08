package chain

import (
	"errors"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
)

type BridgeChain struct {
	proto map[string]bridge.Bridger
}

func NewBridgeChain() *BridgeChain {
	return &BridgeChain{map[string]bridge.Bridger{}}
}

func (b *BridgeChain) Bridge(dialer bridge.Dialer, addr string) (bridge.Dialer, bridge.ListenConfig, error) {
	return b.BridgeChain(dialer, strings.Split(addr, "<")...)
}

func (b *BridgeChain) BridgeChain(dialer bridge.Dialer, addrs ...string) (bridge.Dialer, bridge.ListenConfig, error) {
	if len(addrs) == 0 {
		return dialer, nil, nil
	}
	addr := addrs[len(addrs)-1]
	d, l, err := b.bridge(dialer, addr)
	if err != nil {
		return nil, nil, err
	}
	addrs = addrs[:len(addrs)-1]
	if len(addrs) == 0 {
		return d, l, nil
	}
	return b.BridgeChain(d, addrs...)
}

func (b *BridgeChain) bridge(dialer bridge.Dialer, addr string) (bridge.Dialer, bridge.ListenConfig, error) {
	ur, err := url.Parse(addr)
	if err != nil {
		return nil, nil, err
	}
	dial, ok := b.proto[ur.Scheme]
	if !ok {
		return nil, nil, errors.New("not define " + addr)
	}
	return dial.Bridge(dialer, addr)
}

func (b *BridgeChain) Register(name string, bridger bridge.Bridger) error {
	b.proto[name] = bridger
	return nil
}
