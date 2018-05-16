package ssdb

import (
	"net"

	"github.com/wzshiming/bridge"
	ssdb "github.com/wzshiming/ssdb"
)

// OpenSSDB 创建 ssdb 连接
func OpenSSDB(addr string, bridges ...string) (*ssdb.Client, error) {
	if len(bridges) == 0 {
		return ssdb.ConnectByAddr(addr)
	}

	// 建立代理
	sshcli, err := bridge.BridgeSSH(nil, bridges...)
	if err != nil {
		return nil, err
	}

	// ssdb 连接
	return ssdb.Connect(func() (net.Conn, error) {
		return sshcli.Dial("tcp", addr)
	})
}
