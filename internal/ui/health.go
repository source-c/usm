package ui

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"apps.z7.ai/usm/internal/usm"
)

const (
	timeout = 100 * time.Millisecond
)

// handleConnection handles the connection returning the usm version
func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Send service information to the client and exits
	_, err := conn.Write([]byte(usm.ServiceVersion() + "\n"))
	if err != nil {
		fmt.Println("Error writing server info:", err)
		return
	}
}

// HealthService starts a health service that listens on a random port.
// In the current implementation is used only to avoid starting multiple
// instances of the app.
func HealthService(lockFile string) (net.Listener, error) {
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		log.Println("could not start health service:", err)
		return nil, err
	}
	defer listener.Close()

	err = os.WriteFile(lockFile, []byte(listener.Addr().String()), 0o600)
	if err != nil {
		log.Println("could not write health service lock file:", err)
		return nil, err
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return listener, fmt.Errorf("health service: error accepting connection: %w", err)
		}
		go handleConnection(conn)
	}
}

// HealthServiceCheck checks if the health service is running.
func HealthServiceCheck(lockFile string) bool {
	address, err := os.ReadFile(lockFile) //nolint:gosec // lockFile is application-controlled path
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	d := &net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", string(address))
	if err != nil {
		return false
	}

	// Read the service version from the app and close the connection
	_ = conn.SetReadDeadline(time.Now().UTC().Add(timeout))
	buffer := make([]byte, 4)
	_, err = conn.Read(buffer)
	conn.Close()
	if err != nil {
		return false
	}

	// check for usm service
	return bytes.Equal([]byte(usm.ServicePrefix), buffer)
}
