package driver

import (
	"fmt"
	"net"
)

// TCPPort 封装 TCP 连接作为 Port 接口实现 (用于 Mock 模式)
type TCPPort struct {
	conn net.Conn
}

// Ensure TCPPort implements Port interface
var _ Port = (*TCPPort)(nil)

func OpenTCP(address string) (Port, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mock POS at %s: %v", address, err)
	}
	fmt.Printf("Connected to Mock POS at %s\n", address)
	return &TCPPort{conn: conn}, nil
}

func (t *TCPPort) Read(p []byte) (n int, err error) {
	return t.conn.Read(p)
}

func (t *TCPPort) Write(p []byte) (n int, err error) {
	return t.conn.Write(p)
}

func (t *TCPPort) Close() error {
	return t.conn.Close()
}

func (t *TCPPort) ResetInputBuffer() error {
	// TCP doesn't have a buffer to reset in the same way serial does
	// We could read and discard, but for simplicity we no-op
	return nil
}
