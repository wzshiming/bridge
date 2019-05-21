package httpproxy

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// BasicAuth HTTP Basic authentication for Header Proxy-Authorization
func BasicAuth(username, password string) func(http.ResponseWriter, *http.Request) bool {
	return func(w http.ResponseWriter, r *http.Request) bool {
		if u, p, _ := parseBasicAuth(r.Header.Get("Proxy-Authorization")); u == username && p == password {
			return true
		}
		http.Error(w, "Unauthorized", 407)
		return false
	}

}

// parseBasicAuth parses an HTTP Basic Authentication string.
func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}
