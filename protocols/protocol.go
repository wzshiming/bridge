package protocols

import (
	"fmt"
	"net/url"
	"strings"
)

// Metadata is a map of metadata fields.
type Metadata map[string]string

// Endpoint represents a network endpoint.
type Endpoint struct {
	// Network is the name of the network.
	// stream-like, e.g. tcp, unix, etc.
	// packet-like, e.g. udp, unixpacket, etc.
	Network string `json:"network"`
	// Address is the address of the endpoint.
	// e.g. "localhost:8080", "/tmp/foo.sock"
	Address string `json:"address"`
}

// Wrapper is a network wrapper.
// Wrap a packet-like network with a stream-like network.
// e.g. udp -> quic
// Wrap a stream-like network with a stream-like network.
// e.g. tls or compression
// Wrap a stream-like network with a proxy.
// e.g. tcp -> socks5
type Wrapper struct {
	// Scheme is the scheme of the wrapper.
	Scheme string `json:"scheme"`
	// Metadata is the metadata of the wrapper.
	Metadata Metadata `json:"metadata,omitempty"`
}

// Protocol represents a network protocol.
type Protocol struct {
	// Wrappers is a list of network wrappers.
	Wrappers []Wrapper `json:"wrappers,omitempty"`
	// Endpoint is the endpoint of the protocol.
	Endpoint Endpoint `json:"endpoint"`
}

var ErrInvalidScheme = fmt.Errorf("invalid scheme")

func NewProtocol(rawURI string) (*Protocol, error) {
	uri, err := url.Parse(rawURI)
	if err != nil {
		return nil, err
	}
	return NewProtocolFrom(uri)
}

func NewProtocolFrom(uri *url.URL) (*Protocol, error) {
	protocol := &Protocol{}
	schemes := strings.Split(getReverseAlias(uri.Scheme), "+")

	baseScheme := schemes[len(schemes)-1]
	prevScheme, ok := getSchemeInfo(baseScheme)
	if !ok {
		return nil, fmt.Errorf("unkwon scheme %q: %w", baseScheme, ErrInvalidScheme)
	}

	if prevScheme.Base != KindNone {
		switch prevScheme.Base {
		case KindStream:
			baseScheme = defaultKindStream
		case KindPacket:
			baseScheme = defaultKindPacket
		default:
			return nil, fmt.Errorf("invalid metadata %q: %w", baseScheme, ErrInvalidScheme)
		}
		prevScheme, _ = getSchemeInfo(baseScheme)
		schemes = append(schemes, baseScheme)
	}

	metadata := Metadata{}
	uriQuery := uri.Query()
	for k := range uriQuery {
		metadata[k] = uriQuery.Get(k)
	}

	protocol.Endpoint.Network = baseScheme
	switch prevScheme.AddressKind {
	case AddressOpaque:
		protocol.Endpoint.Address = uri.Opaque
	case AddressHost:
		protocol.Endpoint.Address = uri.Host
	case AddressPath:
		protocol.Endpoint.Address = uri.Path
	}

	if len(schemes) > 1 {
		protocol.Wrappers = make([]Wrapper, len(schemes)-1)
		for i := len(schemes) - 2; i >= 0; i-- {
			curScheme, ok := getSchemeInfo(schemes[i])
			if !ok {
				return nil, fmt.Errorf("unkwon scheme %q: %w", schemes[i], ErrInvalidScheme)
			}
			if curScheme.Base != prevScheme.Kind {
				return nil, fmt.Errorf("invalid combination of scheme %q + %q: %w", schemes[i], schemes[i+1], ErrInvalidScheme)
			}

			wrapper := Wrapper{
				Scheme: schemes[i],
			}
			for _, field := range curScheme.MetaFields {
				if v, ok := metadata[field]; ok {
					if wrapper.Metadata == nil {
						wrapper.Metadata = Metadata{}
					}
					wrapper.Metadata[field] = v
				}
			}
			protocol.Wrappers[i] = wrapper

			prevScheme = curScheme
		}
	}
	if uri.User != nil {
		curScheme, _ := getSchemeInfo(protocol.Wrappers[0].Scheme)
		if curScheme.UsernameField != "" {
			if protocol.Wrappers[0].Metadata == nil {
				protocol.Wrappers[0].Metadata = Metadata{}
			}
			protocol.Wrappers[0].Metadata[curScheme.UsernameField] = uri.User.Username()
			if curScheme.PasswordField != "" {
				if password, ok := uri.User.Password(); ok {
					protocol.Wrappers[0].Metadata[curScheme.PasswordField] = password
				}
			}
		}
	}
	return protocol, nil
}

func (p *Protocol) String() string {
	return p.URI().String()
}

func (p *Protocol) URI() *url.URL {
	uri := &url.URL{}

	networkScheme, _ := getSchemeInfo(p.Endpoint.Network)
	switch networkScheme.AddressKind {
	case AddressOpaque:
		uri.Opaque = p.Endpoint.Address
	case AddressHost:
		uri.Host = p.Endpoint.Address
	case AddressPath:
		uri.Path = p.Endpoint.Address
	}

	if len(p.Wrappers) == 0 {
		uri.Scheme = p.Endpoint.Network
		return uri
	}

	uriQuery := url.Values{}
	schemes := make([]string, 0, len(p.Wrappers))
	for i, wrapper := range p.Wrappers {
		if i == 0 && len(wrapper.Metadata) != 0 {
			curScheme, _ := getSchemeInfo(wrapper.Scheme)
			if username, ok := wrapper.Metadata[curScheme.UsernameField]; ok && curScheme.UsernameField != "" {
				if password, ok := wrapper.Metadata[curScheme.PasswordField]; ok && curScheme.PasswordField != "" {
					uri.User = url.UserPassword(username, password)
				} else {
					uri.User = url.User(username)
				}
			}
			for key, value := range wrapper.Metadata {
				if key == curScheme.UsernameField || key == curScheme.PasswordField {
					continue
				}
				uriQuery.Set(key, value)
			}
		} else {
			for key, value := range wrapper.Metadata {
				uriQuery.Set(key, value)
			}
		}

		schemes = append(schemes, wrapper.Scheme)
	}
	if p.Endpoint.Network != defaultKindStream && p.Endpoint.Network != defaultKindPacket {
		schemes = append(schemes, p.Endpoint.Network)
	}
	uri.Scheme = getAlias(strings.Join(schemes, "+"))
	if len(uriQuery) != 0 {
		uri.RawQuery = uriQuery.Encode()
	}
	return uri
}

var (
	alias        = map[string]string{}
	reverseAlias = map[string]string{}
)

func RegisterAlias(k, v string) {
	alias[k] = v
	reverseAlias[v] = k
}

func getAlias(scheme string) string {
	if v, ok := alias[scheme]; ok {
		return v
	}
	return scheme
}

func getReverseAlias(scheme string) string {
	if v, ok := reverseAlias[scheme]; ok {
		return v
	}
	return scheme
}

type SchemeKind string

var (
	KindNone   SchemeKind = ""
	KindStream SchemeKind = "stream"
	KindPacket SchemeKind = "packet"
	KindProxy  SchemeKind = "proxy"

	defaultKindStream = "tcp"
	defaultKindPacket = "udp"
)

type SchemeInfo struct {
	Kind          SchemeKind
	Base          SchemeKind
	AddressKind   AddressKind
	UsernameField string
	PasswordField string
	MetaFields    []string
	ListenSupport []string
	DialSupport   []string
}

type AddressKind uint8

var (
	AddressHost   AddressKind = 1
	AddressPath   AddressKind = 2
	AddressOpaque AddressKind = 3
)

func getSchemeInfo(scheme string) (SchemeInfo, bool) {
	info, ok := schemeInfoData[scheme]
	return info, ok
}

func RegisterScheme(scheme string, schemeInfo SchemeInfo) {
	schemeInfoData[scheme] = schemeInfo
}

var (
	schemeInfoData = map[string]SchemeInfo{}
)
