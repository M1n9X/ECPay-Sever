package driver

import "io"

// Port defines the serial port interface for RS232 communication
type Port interface {
	io.ReadWriteCloser
	ResetInputBuffer() error
}
