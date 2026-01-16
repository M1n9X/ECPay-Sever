package driver

import (
	"fmt"
	"time"

	"go.bug.st/serial"
)

// RealPort wraps go.bug.st/serial for RS232 communication
type RealPort struct {
	serial.Port
	portName string
}

// Ensure RealPort implements Port interface
var _ Port = (*RealPort)(nil)

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
	// This allows the outer loop to check for cancellation/timeout
	if err := port.SetReadTimeout(100 * time.Millisecond); err != nil {
		port.Close()
		return nil, fmt.Errorf("failed to set read timeout: %v", err)
	}

	fmt.Printf("Serial port %s opened at %d bps (8N1)\n", portName, baudRate)
	return &RealPort{Port: port, portName: portName}, nil
}

// GetPortName returns the port name for logging
func (p *RealPort) GetPortName() string {
	return p.portName
}
