package httpproxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
)

func proxyOther(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = ""
	cli := http.Client{}
	resp, err := cli.Do(r)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	for k, _ := range resp.Header {
		w.Header().Set(k, resp.Header.Get(k))
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	return
}

func proxyConnect(w http.ResponseWriter, r *http.Request) {
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

	targetConn, err := net.Dial("tcp", r.URL.Host)
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
	Pass func(http.ResponseWriter, *http.Request) bool
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.Pass != nil && !p.Pass(w, r) {
		return
	}
	handle := http.NotFound
	if r.Method == "CONNECT" {
		handle = proxyConnect
	} else if r.URL.Host != "" {
		handle = proxyOther
	}
	handle(w, r)
}
