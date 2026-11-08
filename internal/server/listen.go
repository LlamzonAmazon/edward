package server

import (
	"context"
	"fmt"
	"net"
)

func listen(addr string) (net.Listener, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if host != "127.0.0.1" || port != "43393" {
		return nil, fmt.Errorf("refusing to bind %s; only 127.0.0.1:43393 is allowed", addr)
	}
	var lc net.ListenConfig
	return lc.Listen(context.Background(), "tcp", "127.0.0.1:43393")
}

func Listen(addr string) (net.Listener, error) {
	return listen(addr)
}
