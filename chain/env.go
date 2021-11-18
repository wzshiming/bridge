package chain

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/hostmatcher"
)

var (
	NoProxy   hostmatcher.Matcher
	OnlyProxy hostmatcher.Matcher
)

func init() {
	noProxy, ok := os.LookupEnv("no_proxy")
	if !ok {
		noProxy, ok = os.LookupEnv("NO_PROXY")
	}
	if ok && noProxy != "" {
		list := strings.Split(noProxy, ",")
		if len(list) != 0 {
			NoProxy = hostmatcher.NewMatcher(list)
		}
	}

	onlyProxy, ok := os.LookupEnv("only_proxy")
	if !ok {
		onlyProxy, ok = os.LookupEnv("ONLY_PROXY")
	}
	if ok && onlyProxy != "" {
		list := strings.Split(onlyProxy, ",")
		if len(list) != 0 {
			OnlyProxy = hostmatcher.NewMatcher(list)
		}
	}
}

func NewEnvDialer(dialer bridge.Dialer) bridge.Dialer {
	if OnlyProxy == nil && NoProxy == nil {
		return dialer
	}
	if OnlyProxy != nil {
		dialer = NewShuntDialer(local.LOCAL, dialer, OnlyProxy)
	}
	if NoProxy != nil {
		dialer = NewShuntDialer(dialer, local.LOCAL, NoProxy)
	}
	if l, ok := dialer.(bridge.ListenConfig); ok {
		return struct {
			bridge.Dialer
			bridge.ListenConfig
		}{
			dialer,
			l,
		}
	}
	return dialer
}

type shuntDialer struct {
	dialer      bridge.Dialer
	matchDialer bridge.Dialer
	matcher     hostmatcher.Matcher
}

func NewShuntDialer(dialer bridge.Dialer, matchDialer bridge.Dialer, matcher hostmatcher.Matcher) bridge.Dialer {
	if matcher == nil || matchDialer == nil {
		return dialer
	}
	return &shuntDialer{
		dialer:      dialer,
		matchDialer: matchDialer,
		matcher:     matcher,
	}
}

func (s *shuntDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if s.matcher.Match(address) {
		return s.matchDialer.DialContext(ctx, network, address)
	}
	return s.dialer.DialContext(ctx, network, address)
}
