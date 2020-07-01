package ssh

import (
	"context"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/local"
	"golang.org/x/crypto/ssh"
)

// SSH ssh://[username:password@]{address}[?identity_file=path/to/file]
func SSH(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	host, config, err := config(addr)
	if err != nil {
		return nil, err
	}

	cli, err := newClient(dialer, host, config)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

type Client struct {
	mut    sync.Mutex
	dialer bridge.Dialer
	sshCli *ssh.Client
	host   string
	config *ssh.ClientConfig
}

func newClient(dialer bridge.Dialer, host string, config *ssh.ClientConfig) (*Client, error) {
	cli := &Client{
		dialer: dialer,
		host:   host,
		config: config,
	}
	return cli, nil
}

func (c *Client) reset() {
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.sshCli == nil {
		return
	}
	c.sshCli.Close()
	c.sshCli = nil
}
func (c *Client) getCli(ctx context.Context) error {
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.sshCli != nil {
		return nil
	}
	conn, err := c.dialer.DialContext(ctx, "tcp", c.host)
	if err != nil {
		return err
	}

	con, chans, reqs, err := ssh.NewClientConn(conn, c.host, c.config)
	if err != nil {
		return err
	}
	c.sshCli = ssh.NewClient(con, chans, reqs)
	return nil
}

func (c *Client) CommandDialContext(ctx context.Context, name string, args ...string) (net.Conn, error) {
	cmd := make([]string, 0, len(args)+1)
	cmd = append(cmd, name)
	for _, arg := range args {
		cmd = append(cmd, strconv.Quote(arg))
	}
	return c.commandDialContext(ctx, strings.Join(cmd, " "), 1)
}

func (c *Client) commandDialContext(ctx context.Context, cmd string, retry int) (net.Conn, error) {
	err := c.getCli(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := c.sshCli.NewSession()
	if err != nil {
		return nil, err
	}
	conn1, conn2 := net.Pipe()
	sess.Stdin = conn1
	sess.Stdout = conn1
	sess.Stderr = os.Stderr
	err = sess.Start(cmd)
	if err != nil {
		if retry != 0 {
			c.reset()
			return c.commandDialContext(ctx, cmd, retry-1)
		}
		return nil, err
	}
	go func() {
		sess.Wait()
		conn1.Close()
	}()

	return conn2, nil
}

func (c *Client) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return c.dialContext(ctx, network, address, 1)
}

func (c *Client) dialContext(ctx context.Context, network, address string, retry int) (net.Conn, error) {
	err := c.getCli(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := c.sshCli.Dial(network, address)
	if err != nil {
		if retry != 0 {
			c.reset()
			return c.dialContext(ctx, network, address, retry-1)
		}
		return nil, err
	}
	return conn, nil
}

func (c *Client) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return c.listen(ctx, network, address, 1)
}

func (c *Client) listen(ctx context.Context, network, address string, retry int) (net.Listener, error) {
	err := c.getCli(ctx)
	if err != nil {
		return nil, err
	}
	listener, err := c.sshCli.Listen(network, address)
	if err != nil {
		if retry != 0 {
			c.reset()
			return c.listen(ctx, network, address, retry-1)
		}
		return nil, err
	}
	return listener, nil
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
