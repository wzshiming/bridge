package scheme

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func JoinSchemeAddr(sch, addr string) (string, bool) {
	if sch == "" && addr == "" {
		return "", false
	}
	if addr == "" {
		return fmt.Sprintf("%s:", sch), true
	}
	if sch == "" {
		return fmt.Sprintf("tcp://%s", addr), true
	}
	return fmt.Sprintf("%s://%s", sch, addr), true
}

func SplitSchemeAddr(addr string) (string, string, bool) {
	// scheme:
	if strings.HasSuffix(addr, ":") {
		return addr[:len(addr)-1], "", true
	}
	// :port
	if strings.HasPrefix(addr, ":") {
		return "tcp", addr, true
	}
	// ./path/to/socks
	if strings.HasPrefix(addr, "./") || strings.HasPrefix(addr, "/") {
		return "unix", addr, true
	}

	u, _ := url.Parse(addr)
	if u != nil && u.Scheme != "" {
		// scheme://host
		if u.Opaque == "" {
			if u.Host != "" {
				return u.Scheme, u.Host, true
			}
			if u.Path != "" {
				return u.Scheme, u.Path, true
			}
			// scheme:?args=...
			if u.ForceQuery || u.RawQuery != "" {
				return u.Scheme, u.RawQuery, true
			}
		}

		// scheme: other
		if strings.ContainsAny(u.Opaque, " ./") {
			return u.Scheme, strings.TrimSpace(u.Opaque), true
		}

		// ip:port or host:port
		if strings.Contains(u.Scheme, ".") {
			return "tcp", net.JoinHostPort(u.Scheme, strings.TrimSpace(u.Opaque)), true
		}
	}

	if strings.Contains(addr, ":") {
		return "tcp", addr, true
	}

	return "", "", false
}
