package driver

import (
	"fmt"
	"net"
	"time"
)

// TCPPort wraps a TCP connection as a Port interface
// Used for Serial-over-TCP devices or network-attached POS terminals
type TCPPort struct {
	conn    net.Conn
	address string
}

// Ensure TCPPort implements Port interface
var _ Port = (*TCPPort)(nil)

// OpenTCP opens a TCP connection to a POS device
func OpenTCP(address string) (Port, error) {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", address, err)
	}

	fmt.Printf("Connected to POS at %s (TCP)\n", address)
	return &TCPPort{conn: conn, address: address}, nil
}

func (t *TCPPort) Read(p []byte) (n int, err error) {
	// Set read deadline to prevent blocking forever
	t.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	n, err = t.conn.Read(p)

	// Convert timeout to nil error (expected behavior)
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return n, nil
	}
	return n, err
}

func (t *TCPPort) Write(p []byte) (n int, err error) {
	return t.conn.Write(p)
}

func (t *TCPPort) Close() error {
	return t.conn.Close()
}

func (t *TCPPort) ResetInputBuffer() error {
	// Drain any pending data
	buf := make([]byte, 1024)
	t.conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	for {
		n, _ := t.conn.Read(buf)
		if n == 0 {
			break
		}
	}
	return nil
}

// GetAddress returns the TCP address for logging
func (t *TCPPort) GetAddress() string {
	return t.address
}
