package chain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/bridge/logger"
)

// BridgeChain is a bridger that supports multiple crossing of bridger.
type BridgeChain struct {
	DialerFunc   func(dialer bridge.Dialer) bridge.Dialer
	proto        map[string]bridge.Bridger
	defaultProto bridge.Bridger

	backoffCount map[string]uint64
	mutex        sync.Mutex
}

// NewBridgeChain create a new BridgeChain.
func NewBridgeChain() *BridgeChain {
	return &BridgeChain{
		proto:        map[string]bridge.Bridger{},
		DialerFunc:   NewEnvDialer,
		backoffCount: map[string]uint64{},
	}
}

// BridgeChain is multiple crossing of bridge.
func (b *BridgeChain) BridgeChain(ctx context.Context, dialer bridge.Dialer, addresses ...string) (bridge.Dialer, error) {
	if len(addresses) == 0 {
		return dialer, nil
	}
	address := addresses[len(addresses)-1]
	d, err := b.multiDial(dialer, strings.Split(address, "|"))
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
	d, err := b.multiDial(dialer, address.LB)
	if err != nil {
		return nil, err
	}
	addresses = addresses[:len(addresses)-1]
	if len(addresses) == 0 {
		return d, nil
	}
	return b.bridgeChainWithConfig(ctx, d, addresses...)
}

func (b *BridgeChain) multiDial(dialer bridge.Dialer, addresses []string) (bridge.Dialer, error) {
	useCount := &backoffManager{
		addresses: addresses,
		dialer:    dialer,
		bc:        b,
	}
	return useCount, nil
}

func (b *BridgeChain) singleDial(ctx context.Context, dialer bridge.Dialer, address string) (bridge.Dialer, error) {
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

type backoffManager struct {
	addresses []string
	dialer    bridge.Dialer

	bc *BridgeChain
}

func (u *backoffManager) useLeastAndCount(addresses []string) string {
	if len(addresses) == 1 {
		return addresses[0]
	}
	min := uint64(math.MaxUint64)

	u.bc.mutex.Lock()
	defer u.bc.mutex.Unlock()

	var minAddress string
	for _, address := range addresses {
		if u.bc.backoffCount[address] < min {
			min = u.bc.backoffCount[address]
			minAddress = address
		}
	}
	u.bc.backoffCount[minAddress]++
	return minAddress
}

func (u *backoffManager) backoff(address string, count uint64) {
	u.bc.mutex.Lock()
	defer u.bc.mutex.Unlock()
	u.bc.backoffCount[address] += count
}

func (u *backoffManager) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var errs []error

	tryTimes := len(u.addresses)
	for i := 0; i < tryTimes; i++ {
		addr := u.useLeastAndCount(u.addresses)
		dialer, err := u.bc.singleDial(ctx, u.dialer, addr)
		if err != nil {
			errs = append(errs, err)
			logger.Std.Warn("failed dial", "err", err, "previous", addr)
			u.backoff(addr, 16)
			continue
		}
		conn, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			errs = append(errs, err)
			logger.Std.Warn("failed dial target", "err", err, "previous", addr, "target", address)
			u.backoff(addr, 8)
			continue
		}

		logger.Std.Info("success dial target", "previous", addr, "target", address)
		return conn, nil
	}
	return nil, fmt.Errorf("all addresses are failed: %w", errors.Join(errs...))
}
