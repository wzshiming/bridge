package ssdb

import (
	"github.com/wzshiming/bridge"
	ssdb "github.com/wzshiming/gossdb"
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

	conn, err := sshcli.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	// ssdb 连接
	return ssdb.ConnectByConn(conn)
}
