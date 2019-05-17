package ssh

import (
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"net"
	"net/url"

	"github.com/wzshiming/bridge"
	"golang.org/x/crypto/ssh"
)

// SSH ssh://[username:password@]{address}
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
	host := ur.Host

	if dialer == nil {
		dialer = bridge.DialFunc(net.Dial)
	}
	conn, err := dialer.Dial("tcp", host)
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: user,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}

	if isPwd {
		config.Auth = []ssh.AuthMethod{ssh.Password(pwd)}
	} else {
		file, err := ioutil.ReadFile("~/.ssh/id_rsa")
		if err == nil {
			_, keyByte := pem.Decode(file)
			key, err := x509.ParsePKCS8PrivateKey(keyByte)
			if err != nil {
				return nil, err
			}
			signer, err := ssh.NewSignerFromKey(key)
			if err != nil {
				return nil, err
			}
			config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		}
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, host, config)
	if err != nil {
		return nil, err
	}

	return ssh.NewClient(c, chans, reqs), nil
}
