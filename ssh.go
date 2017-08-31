package bridge

import (
	"net"
	"net/url"
	"strings"

	"github.com/wzshiming/ffmt"

	"golang.org/x/crypto/ssh"
)

var bridgeDial map[string]func(n, addr string) (net.Conn, error)

func RegisterBridge(br string, dials ...string) error {
	if bridgeDial == nil {
		bridgeDial = map[string]func(n, addr string) (net.Conn, error){}
	}

	ffmt.Mark(dials)
	cli, err := BridgeSSH(nil, dials...)
	if err != nil {
		return err
	}
	bridgeDial[br] = cli.Dial
	return nil
}

func GetBridge(br string) func(n, addr string) (net.Conn, error) {
	return bridgeDial[br]
}

func BridgeSSH(cli0 *ssh.Client, addrs ...string) (cli *ssh.Client, err error) {

	// 递归结束
	if len(addrs) == 0 {
		return cli0, nil
	}
	addr := addrs[0]

	// 是否是多级连接
	var dial func(network, address string) (net.Conn, error)
	var conn net.Conn
	if cli0 == nil {
		dial = net.Dial
	} else {
		dial = cli0.Dial
	}

	// 补充协议头
	atI := strings.Index(addr, "ssh://")
	if atI != 0 {
		addr = "ssh://" + addr
	}

	// 解析协议
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

	// 建立连接
	conn, err = dial("tcp", host)
	if err != nil {
		return nil, err
	}

	// 协议验证
	config := &ssh.ClientConfig{
		User: user,
	}
	if isPwd {
		config.Auth = []ssh.AuthMethod{ssh.Password(pwd)}
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, host, config)
	if err != nil {
		return nil, err
	}

	// 返回会话
	cli = ssh.NewClient(c, chans, reqs)
	return BridgeSSH(cli, addrs[1:]...)
}
