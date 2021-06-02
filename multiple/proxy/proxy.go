package proxy

import (
	"context"
	"net"
	"net/url"
	"sort"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/cmux"
	"github.com/wzshiming/cmux/pattern"
)

type Proxy struct {
	proxies map[string]*Host
}

func NewProxy(ctx context.Context, addrs []string, dial bridge.Dialer) (*Proxy, error) {
	proxies := map[string]*Host{}
	for _, addr := range addrs {
		u, err := url.Parse(addr)
		if err != nil {
			return nil, err
		}
		host := u.Host

		s, err := newServer(ctx, addr, dial)
		if err != nil {
			return nil, err
		}
		mux, ok := proxies[host]
		if !ok {
			mux = &Host{
				cmux: cmux.NewCMux(),
			}
		}
		patterns := s.Patterns()
		if patterns == nil {
			mux.proxies = append(mux.proxies, s.ProxyURL())
			err = mux.cmux.NotFound(s)
			if err != nil {
				return nil, err
			}
		} else {
			mux.proxies = append(mux.proxies, s.ProxyURL())
			for _, p := range patterns {
				err = mux.cmux.HandlePrefix(s, pattern.Pattern[p]...)
				if err != nil {
					return nil, err
				}
			}
		}
		proxies[u.Host] = mux
	}
	proxy := &Proxy{
		proxies: proxies,
	}
	return proxy, nil
}

func (s *Proxy) Match(addr string) *Host {
	return s.proxies[addr]
}

func (s *Proxy) Hosts() []string {
	hosts := make([]string, 0, len(s.proxies))
	for proxy := range s.proxies {
		hosts = append(hosts, proxy)
	}
	sort.Strings(hosts)
	return hosts
}

type Host struct {
	cmux    *cmux.CMux
	proxies []string
}

func (h *Host) ProxyURLs() []string {
	return h.proxies
}

func (h *Host) ServeConn(conn net.Conn) {
	h.cmux.ServeConn(conn)
}
