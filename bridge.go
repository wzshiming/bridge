package bridge

import (
	"errors"
	"net/url"
	"strings"
)

type BridgeChain map[string]Bridger

func (b BridgeChain) Bridge(dialer Dialer, addr string) (Dialer, error) {
	return b.BridgeChain(dialer, strings.Split(addr, ">")...)
}

func (b BridgeChain) BridgeChain(dialer Dialer, addrs ...string) (Dialer, error) {
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

func (b BridgeChain) bridge(dialer Dialer, addr string) (Dialer, error) {
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
