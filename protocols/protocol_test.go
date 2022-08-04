package protocols

import (
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestProtocol_URI(t *testing.T) {
	tests := []struct {
		name     string
		protocol *Protocol
		want     *url.URL
	}{
		{
			name: "tcp",
			protocol: &Protocol{
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:8080",
				},
			},
			want: &url.URL{
				Host:   "127.0.0.1:8080",
				Scheme: "tcp",
			},
		},
		{
			name: "http",
			protocol: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:8080",
				},
			},
			want: &url.URL{
				Host:   "127.0.0.1:8080",
				Scheme: "http",
			},
		},
		{
			name: "http with username and password",
			protocol: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
						Metadata: Metadata{
							"username": "username",
							"password": "password",
						},
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:8080",
				},
			},
			want: &url.URL{
				Scheme: "http",
				Host:   "127.0.0.1:8080",
				User:   url.UserPassword("username", "password"),
			},
		},
		{
			name: "https",
			protocol: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
					{
						Scheme: "tls",
						Metadata: Metadata{
							"key_data":  "",
							"cert_data": "",
						},
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:8080",
				},
			},
			want: &url.URL{
				Scheme: "https",
				Host:   "127.0.0.1:8080",
				RawQuery: url.Values{
					"cert_data": []string{""},
					"key_data":  []string{""},
				}.Encode(),
			},
		},
		{
			name: "http with unix",
			protocol: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
				},
				Endpoint: Endpoint{
					Network: "unix",
					Address: "/tmp/test.sock",
				},
			},
			want: &url.URL{
				Path:   "/tmp/test.sock",
				Scheme: "http+unix",
			},
		},
		{
			name: "http with command",
			protocol: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
				},
				Endpoint: Endpoint{
					Network: "command",
					Address: "nc %h %p",
				},
			},
			want: &url.URL{
				Opaque: "nc %h %p",
				Scheme: "http+command",
			},
		},
		{
			name: "http with quic",
			protocol: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
					{
						Scheme: "quic",
					},
				},
				Endpoint: Endpoint{
					Network: "udp",
					Address: "127.0.0.1:8080",
				},
			},
			want: &url.URL{
				Host:   "127.0.0.1:8080",
				Scheme: "http+quic",
			},
		},
		{
			name: "ssh with username",
			protocol: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "ssh",
						Metadata: Metadata{
							"username": "username",
						},
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:22",
				},
			},
			want: &url.URL{
				Host:   "127.0.0.1:22",
				Scheme: "ssh",
				User:   url.User("username"),
			},
		},
		{
			name: "ssh with username and key",
			protocol: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "ssh",
						Metadata: Metadata{
							"username":        "username",
							"authorized_data": "",
							"identity_data":   "",
						},
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:22",
				},
			},
			want: &url.URL{
				Host:   "127.0.0.1:22",
				Scheme: "ssh",
				User:   url.User("username"),
				RawQuery: url.Values{
					"authorized_data": {""},
					"identity_data":   {""},
				}.Encode(),
			},
		},
	}
	compareUser := cmp.Comparer(func(x, y *url.Userinfo) bool {
		return x.String() == y.String()
	})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.protocol.URI()
			if diff := cmp.Diff(tt.want, got, compareUser); diff != "" {
				t.Errorf("Protocol.URI() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewProtocolFrom(t *testing.T) {
	tests := []struct {
		name    string
		uri     *url.URL
		want    *Protocol
		wantErr bool
	}{
		{
			name: "tcp",
			uri: &url.URL{
				Scheme: "tcp",
				Host:   "127.0.0.1:8080",
			},
			want: &Protocol{
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:8080",
				},
			},
		},
		{
			name: "http",
			uri: &url.URL{
				Host:   "127.0.0.1:8080",
				Scheme: "http",
			},
			want: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:8080",
				},
			},
		},
		{
			name: "http with username and password",
			uri: &url.URL{
				Scheme: "http",
				Host:   "127.0.0.1:8080",
				User:   url.UserPassword("username", "password"),
			},
			want: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
						Metadata: Metadata{
							"username": "username",
							"password": "password",
						},
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:8080",
				},
			},
		},
		{
			name: "https",
			uri: &url.URL{
				Scheme: "https",
				Host:   "127.0.0.1:8080",
				RawQuery: url.Values{
					"cert_data": []string{""},
					"key_data":  []string{""},
				}.Encode(),
			},
			want: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
					{
						Scheme: "tls",
						Metadata: Metadata{
							"key_data":  "",
							"cert_data": "",
						},
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:8080",
				},
			},
		},
		{
			name: "http with unix",
			uri: &url.URL{
				Path:   "/tmp/test.sock",
				Scheme: "http+unix",
			},
			want: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
				},
				Endpoint: Endpoint{
					Network: "unix",
					Address: "/tmp/test.sock",
				},
			},
		},
		{
			name: "http with command",
			uri: &url.URL{
				Opaque: "nc %h %p",
				Scheme: "http+command",
			},
			want: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
				},
				Endpoint: Endpoint{
					Network: "command",
					Address: "nc %h %p",
				},
			},
		},
		{
			name: "http with quic",
			uri: &url.URL{
				Host:   "127.0.0.1:8080",
				Scheme: "http+quic",
			},
			want: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "http",
					},
					{
						Scheme: "quic",
					},
				},
				Endpoint: Endpoint{
					Network: "udp",
					Address: "127.0.0.1:8080",
				},
			},
		},
		{
			name: "ssh with username",
			uri: &url.URL{
				Host:   "127.0.0.1:22",
				Scheme: "ssh",
				User:   url.User("username"),
			},
			want: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "ssh",
						Metadata: Metadata{
							"username": "username",
						},
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:22",
				},
			},
		},
		{
			name: "ssh with username and key",
			uri: &url.URL{
				Host:   "127.0.0.1:22",
				Scheme: "ssh",
				User:   url.User("username"),
				RawQuery: url.Values{
					"authorized_data": {""},
					"identity_data":   {""},
				}.Encode(),
			},
			want: &Protocol{
				Wrappers: []Wrapper{
					{
						Scheme: "ssh",
						Metadata: Metadata{
							"username":        "username",
							"authorized_data": "",
							"identity_data":   "",
						},
					},
				},
				Endpoint: Endpoint{
					Network: "tcp",
					Address: "127.0.0.1:22",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProtocolFrom(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProtocolFrom() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("NewProtocolFrom() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestConsistency(t *testing.T) {

	tests := []struct {
		name    string
		rawURI  string
		wantErr bool
	}{
		{
			name:    "empty",
			rawURI:  "",
			wantErr: true,
		},
		{
			name:   "tcp",
			rawURI: "tcp://127.0.0.1:1000",
		},
		{
			name:   "http",
			rawURI: "http://127.0.0.1:1000",
		},
		{
			name:   "http with unix",
			rawURI: "http+unix:///tmp/foo.sock",
		},
		{
			name:   "ssh",
			rawURI: "ssh://username@127.0.0.1:22",
		},
		{
			name:    "unknown 1",
			rawURI:  "xxx://xxx",
			wantErr: true,
		},
		{
			name:    "unknown 2",
			rawURI:  "xxx+tcp://xxx",
			wantErr: true,
		},
		{
			name:    "unknown 3",
			rawURI:  "tcp+xxx://xxx",
			wantErr: true,
		},
		{
			name:    "unknown 4",
			rawURI:  "tcp+udp://xxx",
			wantErr: true,
		},
		{
			name:    "unknown 5",
			rawURI:  "xxx+xxx+tcp://xxx",
			wantErr: true,
		},
		{
			name:    "error 1",
			rawURI:  ":",
			wantErr: true,
		},
		{
			name:    "error 2",
			rawURI:  "invalid:",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProtocol(tt.rawURI)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProtocol() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if diff := cmp.Diff(got.String(), tt.rawURI); diff != "" {
					t.Errorf("NewProtocol() and String() mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func init() {
	RegisterAlias("http+tls", "https")
	for scheme, raw := range schemeInfoDataRaw {
		RegisterScheme(scheme, raw)
	}
}

var (
	schemeInfoDataRaw = map[string]SchemeInfo{
		"invalid": {
			Kind: KindNone,
			Base: KindProxy,
		},
		"command": {
			Kind:        KindStream,
			Base:        KindNone,
			AddressKind: AddressOpaque,
		},
		"tcp": {
			Kind:        KindStream,
			Base:        KindNone,
			AddressKind: AddressHost,
		},
		"udp": {
			Kind:        KindPacket,
			Base:        KindNone,
			AddressKind: AddressHost,
		},
		"unix": {
			Kind:        KindStream,
			Base:        KindNone,
			AddressKind: AddressPath,
		},
		"unixgram": {
			Kind:        KindPacket,
			Base:        KindNone,
			AddressKind: AddressPath,
		},
		"snappy": {
			Kind: KindStream,
			Base: KindStream,
		},
		"quic": {
			Kind: KindStream,
			Base: KindPacket,
			MetaFields: []string{
				"key_data",
				"cert_data",
				"ca_data",
			},
		},
		"tls": {
			Kind: KindStream,
			Base: KindStream,
			MetaFields: []string{
				"key_data",
				"cert_data",
				"ca_data",
			},
		},
		"socks5": {
			Kind:          KindProxy,
			Base:          KindStream,
			UsernameField: "username",
			PasswordField: "password",
			ListenSupport: []string{
				"tcp",
			},
			DialSupport: []string{
				"tcp",
			},
		},
		"socks4": {
			Kind:          KindProxy,
			Base:          KindStream,
			UsernameField: "username",
			DialSupport: []string{
				"tcp",
			},
		},
		"http": {
			Kind:          KindProxy,
			Base:          KindStream,
			UsernameField: "username",
			PasswordField: "password",
			DialSupport: []string{
				"tcp",
			},
		},
		"shadowsocks": {
			Kind:          KindProxy,
			Base:          KindStream,
			UsernameField: "username",
			PasswordField: "password",
			DialSupport: []string{
				"tcp",
			},
		},
		"ssh": {
			Kind:          KindProxy,
			Base:          KindStream,
			UsernameField: "username",
			PasswordField: "password",
			MetaFields: []string{
				"authenticate",
				"hostkey_data",
				"authorized_data",
				"identity_data",
			},
			ListenSupport: []string{
				"tcp", "unix", "command",
			},
			DialSupport: []string{
				"tcp", "unix", "command",
			},
		},
	}
)
