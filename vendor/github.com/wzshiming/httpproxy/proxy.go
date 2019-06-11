package httpproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
)

func (p *ProxyHandler) proxyOther(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = ""
	cli := http.Client{}
	if p.ProxyDial != nil {
		tran := &http.Transport{
			DialContext: p.ProxyDial,
		}
		cli.Transport = tran
	}
	resp, err := cli.Do(r)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	for k := range resp.Header {
		w.Header().Set(k, resp.Header.Get(k))
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	return
}

func (p *ProxyHandler) proxyConnect(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not allowed", 500)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var targetConn net.Conn
	if p.ProxyDial != nil {
		targetConn, err = p.ProxyDial(r.Context(), "tcp", r.URL.Host)
	} else {
		targetConn, err = net.Dial("tcp", r.URL.Host)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("net.Dial(%q) failed: %v", r.URL.Host, err), 500)
		return
	}

	go func() {
		defer clientConn.Close()
		io.Copy(targetConn, clientConn)
	}()
	go func() {
		defer targetConn.Close()
		io.Copy(clientConn, targetConn)
	}()
	return
}

// ProxyHandler proxy handler
type ProxyHandler struct {
	ProxyDial      func(context.Context, string, string) (net.Conn, error)
	Authentication Authentication
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.Authentication != nil && !p.Authentication.Auth(w, r) {
		return
	}
	handle := http.NotFound
	if r.Method == "CONNECT" {
		handle = p.proxyConnect
	} else if r.URL.Host != "" {
		handle = p.proxyOther
	}
	handle(w, r)
}
