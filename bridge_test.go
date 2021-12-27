package bridge_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	_ "github.com/wzshiming/bridge/protocols/command"
	_ "github.com/wzshiming/bridge/protocols/connect"
	_ "github.com/wzshiming/bridge/protocols/netcat"
	_ "github.com/wzshiming/bridge/protocols/shadowsocks"
	_ "github.com/wzshiming/bridge/protocols/smux"
	_ "github.com/wzshiming/bridge/protocols/socks4"
	_ "github.com/wzshiming/bridge/protocols/socks5"
	_ "github.com/wzshiming/bridge/protocols/ssh"
	_ "github.com/wzshiming/bridge/protocols/tls"
	_ "github.com/wzshiming/bridge/protocols/ws"

	_ "github.com/wzshiming/anyproxy/proxies/httpproxy"
	_ "github.com/wzshiming/anyproxy/proxies/shadowsocks"
	_ "github.com/wzshiming/anyproxy/proxies/socks4"
	_ "github.com/wzshiming/anyproxy/proxies/socks5"
	_ "github.com/wzshiming/anyproxy/proxies/sshproxy"

	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/internal/log"
)

var ctx = context.Background()

func Bridge(ctx context.Context, listens, dials []string) error {
	return chain.Bridge(ctx, listens, dials, false)
}

func MustProxy(addr string) (uri string) {
	uri, err := newProxy(addr)
	if err != nil {
		panic(err)
	}
	return uri
}

func newProxy(addr string) (uri string, err error) {
	u, err := url.Parse(addr)
	if err != nil {
		return "", err
	}
	proxy, err := anyproxy.NewAnyProxy(ctx, []string{addr}, &net.Dialer{}, nil, nil)
	if err != nil {
		return "", err
	}
	host := proxy.Match(u.Host)
	listener, err := net.Listen("tcp", u.Host)
	if err != nil {
		return "", err
	}
	u.Host = listener.Addr().String()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Std.Println(err)
				return
			}
			go host.ServeConn(conn)
		}
	}()
	return u.String(), nil
}

var ProxyServer = []string{
	"socks5://127.0.0.1:0",
	"socks4://127.0.0.1:0",
	"http://127.0.0.1:0",
	"ssh://127.0.0.1:0",
	"http://h:p@127.0.0.1:0",
	"socks4://s4@127.0.0.1:0",
	"socks5://s5:p@127.0.0.1:0",
	"ssh://s:p@127.0.0.1:0",
}

func init() {
	for i, proxy := range ProxyServer {
		ProxyServer[i] = MustProxy(proxy)
		log.Println(ProxyServer[i])
	}
}

func TestPortForward(t *testing.T) {
	want := "OK"
	ser := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(want))
	}))

	u, err := url.Parse(ser.URL)
	if err != nil {
		t.Fatal(err)
	}

	proxy := getRandomAddress()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		err := Bridge(ctx, []string{proxy}, append([]string{u.Host}, ProxyServer...))
		if err != nil {
			t.Log(err)
		}
	}()

	cli := http.Client{}

	for i := 0; i != 10; i++ {
		resp, e := cli.Get("http://" + proxy)
		if e != nil {
			err = e
			time.Sleep(time.Second)
			continue
		}
		data, e := io.ReadAll(resp.Body)
		if err != nil {
			err = e
			time.Sleep(time.Second)
			continue
		}
		resp.Body.Close()
		if string(data) != want {
			err = fmt.Errorf("want %q, got %q", want, data)
			time.Sleep(time.Second)
			continue
		}
		err = nil
		break
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestPortForwardWithRemoteListen(t *testing.T) {
	want := "OK"
	ser := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(want))
	}))

	u, err := url.Parse(ser.URL)
	if err != nil {
		t.Fatal(err)
	}

	proxy := getRandomAddress()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		err := Bridge(ctx, append([]string{proxy}, ProxyServer...), []string{u.Host})
		if err != nil {
			t.Log(err)
		}
	}()

	cli := http.Client{}

	for i := 0; i != 10; i++ {
		resp, e := cli.Get("http://" + proxy)
		if e != nil {
			err = e
			time.Sleep(time.Second)
			continue
		}
		data, e := io.ReadAll(resp.Body)
		if err != nil {
			err = e
			time.Sleep(time.Second)
			continue
		}
		resp.Body.Close()
		if string(data) != want {
			err = fmt.Errorf("want %q, got %q", want, data)
			time.Sleep(time.Second)
			continue
		}
		err = nil
		break
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxy(t *testing.T) {
	want := "OK"
	ser := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(want))
	}))

	u, err := url.Parse(ser.URL)
	if err != nil {
		t.Fatal(err)
	}

	proxy := getRandomAddress()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		err := Bridge(ctx, []string{proxy}, append([]string{"-"}, ProxyServer...))
		if err != nil {
			t.Log(err)
		}
	}()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(request *http.Request) (*url.URL, error) {
		return url.Parse("http://" + proxy)
	}
	cli := http.Client{
		Transport: transport,
	}
	for i := 0; i != 10; i++ {
		resp, e := cli.Get("http://" + u.Host)
		if e != nil {
			err = e
			time.Sleep(time.Second)
			continue
		}
		data, e := io.ReadAll(resp.Body)
		if err != nil {
			err = e
			time.Sleep(time.Second)
			continue
		}
		resp.Body.Close()
		if string(data) != want {
			err = fmt.Errorf("want %q, got %q", want, data)
			time.Sleep(time.Second)
			continue
		}
		err = nil
		break
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestProxyWithRemoteListen(t *testing.T) {
	want := "OK"
	ser := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(want))
	}))

	u, err := url.Parse(ser.URL)
	if err != nil {
		t.Fatal(err)
	}

	proxy := getRandomAddress()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		err := Bridge(ctx, append([]string{proxy}, ProxyServer...), []string{"-"})
		if err != nil {
			t.Log(err)
		}
	}()

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(request *http.Request) (*url.URL, error) {
		return url.Parse("http://" + proxy)
	}
	cli := http.Client{
		Transport: transport,
	}
	for i := 0; i != 10; i++ {
		resp, e := cli.Get("http://" + u.Host)
		if e != nil {
			err = e
			time.Sleep(time.Second)
			continue
		}
		data, e := io.ReadAll(resp.Body)
		if err != nil {
			err = e
			time.Sleep(time.Second)
			continue
		}
		resp.Body.Close()
		if string(data) != want {
			err = fmt.Errorf("want %q, got %q", want, data)
			time.Sleep(time.Second)
			continue
		}
		err = nil
		break
	}
	if err != nil {
		t.Fatal(err)
	}
}

func getRandomAddress() string {
	addr, err := net.Listen("tcp", ":0")
	if err != nil {
		return ""
	}
	defer addr.Close()
	return addr.Addr().String()
}
