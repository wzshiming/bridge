package ssdb

import (
	"net"

	"github.com/wzshiming/bridge"
	ssdb "github.com/wzshiming/ssdb"
)

// OpenSSDB 创建 ssdb 连接
func OpenSSDB(addr string, bridges ...string) (*ssdb.Client, error) {
	opts := []ssdb.Option{}
	opts = append(opts, ssdb.Url(addr))

	if len(bridges) == 0 {
		return ssdb.Connect(opts...)
	}

	opts = append(opts, ssdb.DialHandler(func(addr string) (net.Conn, error) {
		// 建立代理
		sshcli, err := bridge.BridgeSSH(nil, bridges...)
		if err != nil {
			return nil, err
		}
		return sshcli.Dial("tcp", addr)
	}))

	// ssdb 连接
	return ssdb.Connect(opts...)
}
