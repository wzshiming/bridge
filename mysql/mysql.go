package mysql

import (
	"database/sql"
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

// OpenMysql 创建 mysql 连接
func OpenMysql(addr string, bridges ...string) (*sql.DB, error) {
	if len(bridges) != 0 {
		RegisterBridge("proxy", bridges...)
	}
	db, err := sql.Open("mysql", addr)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return db, nil
}
