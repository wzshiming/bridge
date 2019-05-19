package chain

import (
	"errors"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
)

type BridgeChain map[string]bridge.Bridger

func (b BridgeChain) Bridge(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	return b.BridgeChain(dialer, strings.Split(addr, ">")...)
}

func (b BridgeChain) BridgeChain(dialer bridge.Dialer, addrs ...string) (bridge.Dialer, error) {
	if len(addrs) == 0 {
		return dialer, nil
	}
	addr := addrs[0]
	d, err := b.bridge(dialer, addr)
	if err != nil {
		return nil, err
	}
	return b.BridgeChain(d, addrs[1:]...)
}

func (b BridgeChain) bridge(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	ur, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	dial, ok := b[ur.Scheme]
	if !ok {
		return nil, errors.New("not define " + addr)
	}
	return dial.Bridge(dialer, addr)
}
