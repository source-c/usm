//go:build !windows

package agent

import (
	"context"
	"net"
	"time"
)

// dialWithTimeout dials the unix socket with a timeout
func dialWithTimeout(socketPath string, timeout time.Duration) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return d.DialContext(ctx, "unix", socketPath)
}

// listen listens on the unix socket
func listen(socketPath string) (net.Listener, error) {
	var lc net.ListenConfig
	return lc.Listen(context.Background(), "unix", socketPath)
}
