package driver

import (
	"fmt"
	"time"

	"go.bug.st/serial"
)

// RealPort 封装 go.bug.st/serial
type RealPort struct {
	serial.Port
}

// Ensure RealPort implements Port interface
var _ Port = (*RealPort)(nil)

func OpenSerial(portName string, baudRate int) (Port, error) {
	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	// Intercept MOCK_POS for testing Auto-Detection logic
	if portName == "MOCK_POS" {
		fmt.Println("Connecting to Mock POS (TCP)...")
		return OpenTCP("localhost:9999")
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}

	// 设置读取超时，防止 Read 永久阻塞导致外层循环无法检查超时
	if err := port.SetReadTimeout(100 * time.Millisecond); err != nil {
		port.Close()
		return nil, fmt.Errorf("failed to set read timeout: %v", err)
	}

	fmt.Printf("Serial port %s opened at %d bps\n", portName, baudRate)
	return &RealPort{Port: port}, nil
}
