package emux

import (
	"context"
	"net/url"
	"strconv"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/pool"
	"github.com/wzshiming/bridge/logger"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/emux"
)

// EMux emux:?handshake=EMUX%20
func EMux(ctx context.Context, dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	handshake, instruction, err := parseConfig(addr)
	if err != nil {
		return nil, err
	}
	d := emux.NewDialer(ctx, dialer)
	d.Instruction = *instruction
	d.BytesPool = pool.Bytes
	d.Logger = logger.Wrap(logger.Std, "emux")
	if handshake != nil {
		if len(handshake) == 0 {
			d.Handshake = nil
		} else {
			d.Handshake = emux.NewHandshake(handshake, true)
		}
	}
	if listenConfig, ok := dialer.(bridge.ListenConfig); ok {
		l := emux.NewListenConfig(ctx, listenConfig)
		l.Instruction = *instruction
		l.BytesPool = pool.Bytes
		l.Logger = logger.Wrap(logger.Std, "emux")
		if handshake != nil {
			if len(handshake) == 0 {
				d.Handshake = nil
			} else {
				l.Handshake = emux.NewHandshake(handshake, false)
			}
		}
		return struct {
			bridge.Dialer
			bridge.ListenConfig
		}{
			Dialer:       d,
			ListenConfig: l,
		}, nil
	}
	return d, nil
}

func parseConfig(addr string) ([]byte, *emux.Instruction, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, nil, err
	}
	var handshake []byte
	instruction := emux.DefaultInstruction
	query := u.Query()
	for key := range query {
		switch key {
		case "handshake":
			handshake = []byte(query.Get(key))
		case "close":
			u, err := strconv.ParseUint(query.Get(key), 0, 8)
			if err != nil {
				return nil, nil, err
			}
			instruction.Close = uint8(u)
		case "connect":
			u, err := strconv.ParseUint(query.Get(key), 0, 8)
			if err != nil {
				return nil, nil, err
			}
			instruction.Connect = uint8(u)
		case "connected":
			u, err := strconv.ParseUint(query.Get(key), 0, 8)
			if err != nil {
				return nil, nil, err
			}
			instruction.Connected = uint8(u)
		case "disconnect":
			u, err := strconv.ParseUint(query.Get(key), 0, 8)
			if err != nil {
				return nil, nil, err
			}
			instruction.Disconnect = uint8(u)
		case "disconnected":
			u, err := strconv.ParseUint(query.Get(key), 0, 8)
			if err != nil {
				return nil, nil, err
			}
			instruction.Disconnected = uint8(u)
		case "data":
			u, err := strconv.ParseUint(query.Get(key), 0, 8)
			if err != nil {
				return nil, nil, err
			}
			instruction.Data = uint8(u)
		}
	}

	return handshake, &instruction, nil
}
