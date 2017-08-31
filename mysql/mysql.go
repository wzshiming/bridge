package mysql

import (
	"net"

	"github.com/go-sql-driver/mysql"
	"github.com/wzshiming/bridge"
)

func RegisterBridge(br string, dial ...string) {
	mysql.RegisterDial(br, func(addr string) (net.Conn, error) {
		err := bridge.RegisterBridge(br, dial...)
		if err != nil {
			return nil, err
		}
		return bridge.GetBridge(br)("tcp", addr)
	})
}
