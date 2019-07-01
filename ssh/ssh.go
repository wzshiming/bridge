package ssh

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/wzshiming/bridge"
	"golang.org/x/crypto/ssh"
)

// SSH ssh://[username:password@]{address}[?identity_file=path/to/file]
func SSH(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	ur, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	user := ""
	pwd := ""
	isPwd := false
	if ur.User != nil {
		user = ur.User.Username()
		pwd, isPwd = ur.User.Password()
	}
	host := ur.Hostname()
	port := ur.Port()
	if port == "" {
		port = "22"
	}
	host += ":" + port

	config := &ssh.ClientConfig{
		User: user,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	if isPwd {
		config.Auth = append(config.Auth, ssh.Password(pwd))
	}

	for _, ident := range ur.Query()["identity_file"] {
		if ident != "" {
			if strings.HasPrefix(ident, "~") {
				home, err := os.UserHomeDir()
				if err == nil {
					ident = filepath.Join(home, ident[1:])
				}
			}
			file, err := ioutil.ReadFile(ident)
			if err == nil {
				key, err := parsePrivateKey(file)
				if err != nil {
					return nil, err
				}
				signer, err := ssh.NewSignerFromKey(key)
				if err != nil {
					return nil, err
				}
				config.Auth = append(config.Auth, ssh.PublicKeys(signer))
			}
		}
	}

	if dialer == nil {
		var d net.Dialer
		dialer = bridge.DialFunc(d.DialContext)
	}

	return bridge.DialFunc(func(ctx context.Context, network, addr string) (c net.Conn, err error) {
		conn, err := dialer.DialContext(ctx, network, host)
		if err != nil {
			return nil, err
		}

		con, chans, reqs, err := ssh.NewClientConn(conn, host, config)
		if err != nil {
			return nil, err
		}

		cli := ssh.NewClient(con, chans, reqs)
		return cli.Dial(network, addr)
	}), nil

}

func parsePrivateKey(privKey []byte) (interface{}, error) {
	var block, _ = pem.Decode(privKey)
	if block == nil {
		return nil, errors.New("Is not a valid private key")
	}
	var priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		var priv, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return priv, nil
	}
	return priv, nil
}
