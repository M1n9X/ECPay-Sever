package driver

import (
	"fmt"
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

// SerialPort wraps go.bug.st/serial for RS232 communication
type SerialPort struct {
	serial.Port
	portName string
}

var _ Port = (*SerialPort)(nil)

// OpenSerial opens a serial port with ECPay POS parameters
// Parameters per RS232 spec: 115200 bps, 8N1, no flow control
func OpenSerial(portName string, baudRate int) (Port, error) {
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
