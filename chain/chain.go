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

// BridgeChain crosses multiple hops, each one a permuteproxy URI.
type BridgeChain struct {
	DialerFunc func(dialer bridge.Dialer) bridge.Dialer
}

// NewBridgeChain create a new BridgeChain.
func NewBridgeChain() *BridgeChain {
	return &BridgeChain{
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
	return newBackoffManager(dialer, permuteProxy, addresses)
}

type backoffManager struct {
	addresses []string
	logAddrs  []string
	dialers   []bridge.Dialer

	baseDialer bridge.Dialer

	bridgeFunc bridge.BridgeFunc

	backoffCount map[int]uint64

	mut sync.Mutex
}

func newBackoffManager(baseDialer bridge.Dialer, bridgeFunc bridge.BridgeFunc, addresses []string) *backoffManager {
	logAddrs := make([]string, len(addresses))
	for i, address := range addresses {
		logAddrs[i] = logAddr(address)
	}
	return &backoffManager{
		addresses:    addresses,
		logAddrs:     logAddrs,
		dialers:      make([]bridge.Dialer, len(addresses)),
		baseDialer:   baseDialer,
		bridgeFunc:   bridgeFunc,
		backoffCount: map[int]uint64{},
	}
}

func logAddr(address string) string {
	sch, addr, ok := scheme.SplitSchemeAddr(address)
	if !ok {
		return ""
	}
	p, ok := scheme.JoinSchemeAddr(sch, addr)
	if !ok || strings.Contains(p, "@") {
		return ""
	}
	return p
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
	previous := u.logAddrs[index]
	dialer := u.dialers[index]
	u.mut.Unlock()

	if dialer == nil {
		d, err := u.bridgeFunc(ctx, u.baseDialer, addr)
		if err != nil {
			logger.Std.Warn("failed dial", "err", err, "previous", previous)
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
		logger.Std.Warn("failed dial target", "err", err, "previous", previous, "target", address)
		u.backoff(index, 8)
		return nil, err
	}

	logger.Std.Info("success dial target", "previous", previous, "target", address)
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

func (u *backoffManager) listen(ctx context.Context, network, address string) (net.Listener, error) {
	u.mut.Lock()
	index := u.useLeastIndex()
	addr := u.addresses[index]
	previous := u.logAddrs[index]
	dialer := u.dialers[index]
	u.mut.Unlock()

	if dialer == nil {
		d, err := u.bridgeFunc(ctx, u.baseDialer, addr)
		if err != nil {
			logger.Std.Warn("failed dial", "err", err, "previous", previous)
			u.backoff(index, 16)
			return nil, err
		}
		dialer = d

		u.mut.Lock()
		u.dialers[index] = d
		u.mut.Unlock()
	}

	l, ok := dialer.(bridge.ListenConfig)
	if !ok || l == nil {
		err := fmt.Errorf("the previous proxy %T could not listen", dialer)
		logger.Std.Warn("failed listen", "err", err, "previous", previous)
		u.backoff(index, 8)
		return nil, err
	}

	listener, err := l.Listen(ctx, network, address)
	if err != nil {
		logger.Std.Warn("failed listen target", "err", err, "previous", previous, "target", address)
		u.backoff(index, 8)
		return nil, err
	}

	logger.Std.Info("success listen target", "previous", previous, "target", address)
	return listener, nil
}

func (u *backoffManager) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	var errs []error
	tryTimes := len(u.addresses)/2 + 1
	for i := 0; i < tryTimes; i++ {
		listener, err := u.listen(ctx, network, address)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return listener, nil
	}
	return nil, errors.Join(errs...)
}
