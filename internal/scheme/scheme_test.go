package scheme

import (
	"testing"
)

func Test_ResolveProtocol(t *testing.T) {
	type args struct {
		addr string
	}
	tests := []struct {
		name        string
		args        args
		wantNetwork string
		wantAddress string
		wantOk      bool
	}{
		{
			args: args{
				addr: "nc:",
			},
			wantNetwork: "nc",
			wantAddress: "",
			wantOk:      true,
		},
		{
			args: args{
				addr: "nc:?",
			},
			wantNetwork: "nc",
			wantAddress: "",
			wantOk:      true,
		},
		{
			args: args{
				addr: "nc:cmd --",
			},
			wantNetwork: "nc",
			wantAddress: "cmd --",
			wantOk:      true,
		},
		{
			args: args{
				addr: "cmd: nc %h %p",
			},
			wantNetwork: "cmd",
			wantAddress: "nc %h %p",
			wantOk:      true,
		},
		{
			args: args{
				addr: ":1111",
			},
			wantNetwork: "tcp",
			wantAddress: ":1111",
			wantOk:      true,
		},
		{
			args: args{
				addr: "tcp://:1111",
			},
			wantNetwork: "tcp",
			wantAddress: ":1111",
			wantOk:      true,
		},
		{
			args: args{
				addr: "domain:1111",
			},
			wantNetwork: "tcp",
			wantAddress: "domain:1111",
			wantOk:      true,
		},
		{
			args: args{
				addr: "tcp://domain:1111",
			},
			wantNetwork: "tcp",
			wantAddress: "domain:1111",
			wantOk:      true,
		},
		{
			args: args{
				addr: "domain.local:1111",
			},
			wantNetwork: "tcp",
			wantAddress: "domain.local:1111",
			wantOk:      true,
		},
		{
			args: args{
				addr: "tcp://domain.local:1111",
			},
			wantNetwork: "tcp",
			wantAddress: "domain.local:1111",
			wantOk:      true,
		},
		{
			args: args{
				addr: "127.0.0.1:1111",
			},
			wantNetwork: "tcp",
			wantAddress: "127.0.0.1:1111",
			wantOk:      true,
		},
		{
			args: args{
				addr: "tcp://127.0.0.1:1111",
			},
			wantNetwork: "tcp",
			wantAddress: "127.0.0.1:1111",
			wantOk:      true,
		},
		{
			args: args{
				addr: "./xxx.socks",
			},
			wantNetwork: "unix",
			wantAddress: "./xxx.socks",
			wantOk:      true,
		},
		{
			args: args{
				addr: "unix:./xxx.socks",
			},
			wantNetwork: "unix",
			wantAddress: "./xxx.socks",
			wantOk:      true,
		},
		{
			args: args{
				addr: "unix:/xxx.socks",
			},
			wantNetwork: "unix",
			wantAddress: "/xxx.socks",
			wantOk:      true,
		},
		{
			args: args{
				addr: "ssh://username@my_server?identity_file=~/.ssh/id_rsa",
			},
			wantNetwork: "ssh",
			wantAddress: "my_server",
			wantOk:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNetwork, gotAddress, gotOk := SplitSchemeAddr(tt.args.addr)
			if gotNetwork != tt.wantNetwork {
				t.Errorf("SplitSchemeAddr() gotNetwork = %v, want %v", gotNetwork, tt.wantNetwork)
			}
			if gotAddress != tt.wantAddress {
				t.Errorf("SplitSchemeAddr() gotAddress = %v, want %v", gotAddress, tt.wantAddress)
			}
			if gotOk != tt.wantOk {
				t.Errorf("SplitSchemeAddr() wantOk = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}
