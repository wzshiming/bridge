package ssh

import (
	"context"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wzshiming/bridge"
	"golang.org/x/crypto/ssh"
)

// SSH ssh://[username:password@]{address}[?identity_file=path/to/file]
func SSH(dialer bridge.Dialer, addr string) (bridge.Dialer, bridge.ListenConfig, error) {
	if dialer == nil {
		var d net.Dialer
		dialer = bridge.DialFunc(d.DialContext)
	}
	host, config, err := config(addr)
	if err != nil {
		return nil, nil, err
	}

	return bridge.DialFunc(func(ctx context.Context, network, addr string) (c net.Conn, err error) {
			cli, err := getCli(dialer, network, host, config)
			if err != nil {
				return nil, err
			}
			conn, err := cli.Dial(network, addr)
			if err != nil {
				resetCli()
				cli, err := getCli(dialer, network, host, config)
				if err != nil {
					return nil, err
				}

				conn, err = cli.Dial(network, addr)
				if err != nil {
					resetCli()
					return nil, err
				}
			}
			return conn, nil
		}), bridge.ListenConfigFunc(func(ctx context.Context, network, addr string) (net.Listener, error) {
			cli, err := getCli(dialer, network, host, config)
			if err != nil {
				return nil, err
			}
			listener, err := cli.Listen(network, addr)
			if err != nil {
				resetCli()
				cli, err := getCli(dialer, network, host, config)
				if err != nil {
					return nil, err
				}
				listener, err = cli.Listen(network, addr)
				if err != nil {
					resetCli()
					return nil, err
				}
			}
			return listener, nil
		}), nil
}

var (
	mut sync.Mutex
	cli *ssh.Client
)

func resetCli() {
	mut.Lock()
	defer mut.Unlock()
	if cli == nil {
		return
	}
	cli.Close()
	cli = nil
}

func getCli(dialer bridge.Dialer, network string, host string, config *ssh.ClientConfig) (*ssh.Client, error) {
	mut.Lock()
	defer mut.Unlock()
	if cli != nil {
		return cli, nil
	}
	conn, err := dialer.DialContext(context.Background(), network, host)
	if err != nil {
		return nil, err
	}

	con, chans, reqs, err := ssh.NewClientConn(conn, host, config)
	if err != nil {
		return nil, err
	}
	cli = ssh.NewClient(con, chans, reqs)
	return cli, nil
}

func config(addr string) (host string, config *ssh.ClientConfig, err error) {
	ur, err := url.Parse(addr)
	if err != nil {
		return "", nil, err
	}

	user := ""
	pwd := ""
	isPwd := false
	if ur.User != nil {
		user = ur.User.Username()
		pwd, isPwd = ur.User.Password()
	}
	host = ur.Hostname()
	port := ur.Port()
	if port == "" {
		port = "22"
	}
	host += ":" + port

	config = &ssh.ClientConfig{
		User: user,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	if isPwd {
		config.Auth = append(config.Auth, ssh.Password(pwd))
	}

	identityFiles := ur.Query()["identity_file"]
	for _, ident := range identityFiles {
		if ident != "" {
			if strings.HasPrefix(ident, "~") {
				home, err := os.UserHomeDir()
				if err == nil {
					ident = filepath.Join(home, ident[1:])
				}
			}
			file, err := ioutil.ReadFile(ident)
			if err == nil {
				signer, err := ssh.ParsePrivateKey(file)
				if err != nil {
					return "", nil, err
				}
				config.Auth = append(config.Auth, ssh.PublicKeys(signer))
			}
		}
	}
	return host, config, nil
}
