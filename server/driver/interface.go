package driver

import "io"

// Port 定义串口操作接口，用于解耦物理串口与 Mock 实现
type Port interface {
	io.ReadWriteCloser
	ResetInputBuffer() error
}
