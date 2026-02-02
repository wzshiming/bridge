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
	d := b.multiDial(dialer, strings.Split(address, "|"))

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

	d := b.multiDial(dialer, address.LB)

	addresses = addresses[:len(addresses)-1]
	if len(addresses) == 0 {
		return d, nil
	}
	return b.bridgeChainWithConfig(ctx, d, addresses...)
}

func (b *BridgeChain) multiDial(dialer bridge.Dialer, addresses []string) bridge.Dialer {
	return newBackoffManager(dialer, b.singleDial, addresses)
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
	dialers   []bridge.Dialer

	baseDialer bridge.Dialer

	bridgeFunc bridge.BridgeFunc

	backoffCount map[int]uint64

	mut sync.Mutex
}

func newBackoffManager(baseDialer bridge.Dialer, bridgeFunc bridge.BridgeFunc, addresses []string) *backoffManager {
	return &backoffManager{
		addresses:    addresses,
		dialers:      make([]bridge.Dialer, len(addresses)),
		baseDialer:   baseDialer,
		bridgeFunc:   bridgeFunc,
		backoffCount: map[int]uint64{},
	}
}

func (u *backoffManager) useLeastIndex() int {
	min := uint64(math.MaxUint64)

	var index int
	for i := range u.addresses {
		if u.backoffCount[i] < min {
			min = u.backoffCount[i]
			index = i
		}
	}

	u.backoffCount[index]++

	if min > math.MaxInt32 {
		for i := range u.backoffCount {
			u.backoffCount[i] -= math.MaxInt32
		}
	}
	return index
}

func (u *backoffManager) backoff(index int, count uint64) {
	u.mut.Lock()
	defer u.mut.Unlock()
	u.backoffCount[index] += count
}

func (u *backoffManager) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	u.mut.Lock()
	index := u.useLeastIndex()
	addr := u.addresses[index]
	dialer := u.dialers[index]
	u.mut.Unlock()

	if dialer == nil {
		d, err := u.bridgeFunc(ctx, u.baseDialer, addr)
		if err != nil {
			logger.Std.Warn("failed dial", "err", err, "previous", addr)
			u.backoff(index, 16)
			return nil, err
		}
		dialer = d

		u.mut.Lock()
		u.dialers[index] = d
		u.mut.Unlock()
	}

	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		logger.Std.Warn("failed dial target", "err", err, "previous", addr, "target", address)
		u.backoff(index, 8)
		return nil, err
	}

	logger.Std.Info("success dial target", "previous", addr, "target", address)
	return conn, nil
}

func (u *backoffManager) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var errs []error
	tryTimes := len(u.addresses)/2 + 1
	for i := 0; i < tryTimes; i++ {
		conn, err := u.dialContext(ctx, network, address)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return conn, nil
	}
	return nil, errors.Join(errs...)
}
