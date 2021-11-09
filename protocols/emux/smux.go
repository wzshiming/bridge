package emux

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/emux"
)

func EMux(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	if listenConfig, ok := dialer.(bridge.ListenConfig); ok {
		return struct {
			bridge.Dialer
			bridge.ListenConfig
		}{
			Dialer:       emux.NewDialer(dialer),
			ListenConfig: emux.NewListenConfig(listenConfig),
		}, nil
	}
	return emux.NewDialer(dialer), nil
}
