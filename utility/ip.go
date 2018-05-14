package utility

import (
	"net"
	"net/http"
)

// ExtractIP calls RemoteAddr(r), and passes the result into net.ParseIP
// and returns that. If the request does not have a valid IP remote address,
// this function will return nil.
func ExtractIP(r *http.Request) net.IP {
	return net.ParseIP(RemoteAddr(r))
}
