package chain

import (
	"fmt"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/schedialer"
	"github.com/wzshiming/schedialer/plugins/probe"
	"github.com/wzshiming/schedialer/plugins/random"
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
	d, err := b.Dial(dialer, strings.Split(address, "|"), "")
	if err != nil {
		return nil, err
	}
	addresses = addresses[:len(addresses)-1]
	if len(addresses) == 0 {
		return d, nil
	}
	return b.BridgeChain(d, addresses...)
}

// BridgeChainWithConfig is multiple crossing of bridge.
func (b *BridgeChain) BridgeChainWithConfig(dialer bridge.Dialer, addresses ...config.Node) (bridge.Dialer, error) {
	if len(addresses) == 0 {
		return dialer, nil
	}
	address := addresses[len(addresses)-1]
	d, err := b.Dial(dialer, address.LB, address.Probe)
	if err != nil {
		return nil, err
	}
	addresses = addresses[:len(addresses)-1]
	if len(addresses) == 0 {
		return d, nil
	}
	return b.BridgeChainWithConfig(d, addresses...)
}

func (b *BridgeChain) Dial(dialer bridge.Dialer, addresses []string, probeUrl string) (bridge.Dialer, error) {
	if len(addresses) == 1 {
		return b.dialOne(dialer, addresses[0])
	}
	plugins := []schedialer.Plugin{
		random.NewRandom(),
	}
	if probeUrl != "" {
		plugins = append(plugins, probe.NewProbe(probeUrl))
	}
	return b.dialMulti(dialer, addresses, plugins)
}

func (b *BridgeChain) dialMulti(dialer bridge.Dialer, addresses []string, plugins []schedialer.Plugin) (bridge.Dialer, error) {
	if len(addresses) == 1 {
		return b.dialOne(dialer, addresses[0])
	}

	plugin := schedialer.NewPlugins(plugins...)
	for _, address := range addresses {
		dial, err := b.dialOne(dialer, address)
		if err != nil {
			return nil, err
		}
		proxy := schedialer.Proxy{
			Name:   address,
			Dialer: dial,
		}
		plugin.AddProxy(&proxy)
	}
	return schedialer.NewSchedialer(plugin), nil
}

func (b *BridgeChain) dialOne(dialer bridge.Dialer, address string) (bridge.Dialer, error) {
	sch, _, ok := scheme.SplitSchemeAddr(address)
	if !ok {
		return nil, fmt.Errorf("unsupported protocol format %q", address)
	}
	bridger, ok := b.proto[sch]
	if !ok {
		return nil, fmt.Errorf("unsupported protocol %q", sch)
	}
	return bridger.Bridge(dialer, address)
}

// Register is register a new bridger for BridgeChain.
func (b *BridgeChain) Register(name string, bridger bridge.Bridger) error {
	b.proto[name] = bridger
	return nil
}
