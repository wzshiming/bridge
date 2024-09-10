package chain

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/schedialer"
	"github.com/wzshiming/schedialer/plugins/probe"
	"github.com/wzshiming/schedialer/plugins/roundrobin"
)

// BridgeChain is a bridger that supports multiple crossing of bridger.
type BridgeChain struct {
	DialerFunc   func(dialer bridge.Dialer) bridge.Dialer
	proto        map[string]bridge.Bridger
	defaultProto bridge.Bridger
}

// NewBridgeChain create a new BridgeChain.
func NewBridgeChain() *BridgeChain {
	return &BridgeChain{
		proto:      map[string]bridge.Bridger{},
		DialerFunc: NewEnvDialer,
	}
}

// BridgeChain is multiple crossing of bridge.
func (b *BridgeChain) BridgeChain(ctx context.Context, dialer bridge.Dialer, addresses ...string) (bridge.Dialer, error) {
	if len(addresses) == 0 {
		return dialer, nil
	}
	address := addresses[len(addresses)-1]
	d, err := b.Dial(ctx, dialer, strings.Split(address, "|"), "")
	if err != nil {
		return nil, err
	}
	addresses = addresses[:len(addresses)-1]
	if len(addresses) == 0 {
		return d, nil
	}
	return b.BridgeChain(ctx, d, addresses...)
}

// BridgeChainWithConfig is multiple crossing of bridge.
func (b *BridgeChain) BridgeChainWithConfig(ctx context.Context, dialer bridge.Dialer, addresses ...config.Node) (bridge.Dialer, error) {
	if len(addresses) == 0 {
		return dialer, nil
	}
	d, err := b.bridgeChainWithConfig(ctx, dialer, addresses...)
	if err != nil {
		return nil, err
	}
	if b.DialerFunc != nil {
		d = b.DialerFunc(d)
	}
	return d, nil
}
func (b *BridgeChain) bridgeChainWithConfig(ctx context.Context, dialer bridge.Dialer, addresses ...config.Node) (bridge.Dialer, error) {
	if len(addresses) == 0 {
		return dialer, nil
	}
	address := addresses[len(addresses)-1]
	d, err := b.Dial(ctx, dialer, address.LB, address.Probe)
	if err != nil {
		return nil, err
	}
	addresses = addresses[:len(addresses)-1]
	if len(addresses) == 0 {
		return d, nil
	}
	return b.bridgeChainWithConfig(ctx, d, addresses...)
}

func (b *BridgeChain) Dial(ctx context.Context, dialer bridge.Dialer, addresses []string, probeUrl string) (bridge.Dialer, error) {
	if len(addresses) == 1 {
		return b.dialOne(ctx, dialer, addresses[0])
	}
	plugins := []schedialer.Plugin{
		roundrobin.NewRoundRobinWithIndex(100, rand.Uint64()%uint64(len(addresses))),
	}
	if probeUrl != "" {
		plugins = append(plugins, probe.NewProbe(100, probeUrl))
	}
	plugin := schedialer.NewPlugins(plugins...)
	for _, address := range addresses {
		dial, err := b.dialOne(ctx, dialer, address)
		if err != nil {
			return nil, err
		}
		proxy := schedialer.Proxy{
			Name:   address,
			Dialer: dial,
		}
		plugin.AddProxy(ctx, &proxy)
	}
	return schedialer.NewSchedialer(plugin), nil
}

func (b *BridgeChain) dialOne(ctx context.Context, dialer bridge.Dialer, address string) (bridge.Dialer, error) {
	sch, _, ok := scheme.SplitSchemeAddr(address)
	if !ok {
		return nil, fmt.Errorf("unsupported protocol format %q", address)
	}
	bridger, ok := b.proto[sch]
	if !ok {
		if b.defaultProto == nil {
			return nil, fmt.Errorf("unsupported protocol %q", sch)
		}
		bridger = b.defaultProto
	}
	return bridger.Bridge(ctx, dialer, address)
}

// Register is register a new bridger for BridgeChain.
func (b *BridgeChain) Register(name string, bridger bridge.Bridger) error {
	b.proto[name] = bridger
	return nil
}

// RegisterDefault is register a default bridger for BridgeChain.
func (b *BridgeChain) RegisterDefault(bridger bridge.Bridger) {
	b.defaultProto = bridger
}
