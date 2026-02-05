package driver

import (
	"fmt"
	"net"
	"strings"
	"time"

	"go.bug.st/serial"
)

// Port defines the serial port interface for RS232 communication
type Port interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	ResetInputBuffer() error
}

// ============================================================================
// Serial Port (Physical RS232)
// ============================================================================

// SerialPort wraps go.bug.st/serial for RS232 communication
type SerialPort struct {
	serial.Port
	portName string
}

var _ Port = (*SerialPort)(nil)

// openSerialPort opens a physical serial port
func openSerialPort(portName string, baudRate int) (Port, error) {
	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}

	// Set read timeout to prevent blocking forever
	if err := port.SetReadTimeout(100 * time.Millisecond); err != nil {
		port.Close()
		return nil, fmt.Errorf("failed to set read timeout: %v", err)
	}

	fmt.Printf("Serial port %s opened at %d bps (8N1)\n", portName, baudRate)
	return &SerialPort{Port: port, portName: portName}, nil
}

func (p *SerialPort) GetPortName() string {
	return p.portName
}

// ============================================================================
// TCP Port (for Mock POS or Serial-over-TCP devices)
// ============================================================================

// TCPPort wraps a TCP connection as a Port interface
type TCPPort struct {
	conn    net.Conn
	address string
}

var _ Port = (*TCPPort)(nil)

// openTCPPort opens a TCP connection
func openTCPPort(address string) (Port, error) {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", address, err)
	}

	fmt.Printf("Connected to %s (TCP)\n", address)
	return &TCPPort{conn: conn, address: address}, nil
}

func (t *TCPPort) Read(p []byte) (n int, err error) {
	t.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	n, err = t.conn.Read(p)
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return n, nil // Timeout is expected
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

// ============================================================================
// Unified Open Function
// ============================================================================

// OpenSerial opens a port - either physical serial or TCP based on the address format
// TCP addresses should be in format: "tcp://host:port"
// Serial ports: "COM3", "/dev/ttyUSB0", etc.
func OpenSerial(portName string, baudRate int) (Port, error) {
	if strings.HasPrefix(portName, "tcp://") {
		addr := strings.TrimPrefix(portName, "tcp://")
		return openTCPPort(addr)
	}
	return openSerialPort(portName, baudRate)
}
