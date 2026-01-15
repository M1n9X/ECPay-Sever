package driver

import (
	"bytes"
	"ecpay-server/protocol"
	"errors"
	"fmt"
	"time"
)

type SerialManager struct {
	Port Port
}

func NewSerialManager(port Port) *SerialManager {
	return &SerialManager{Port: port}
}

// ExecuteTransaction 执行一笔完整的 ECPay 交易
// 流程: Send -> Wait ACK -> Wait Response -> Send ACK
func (sm *SerialManager) ExecuteTransaction(req protocol.ECPayRequest) (map[string]string, error) {
	// 1. 构建数据包
	packet := protocol.BuildPacket(req)

	// 2. 清空输入缓冲
	sm.Port.ResetInputBuffer()

	// 3. 发送数据
	_, err := sm.Port.Write(packet)
	if err != nil {
		return nil, fmt.Errorf("write error: %v", err)
	}

	// 4. 等待 ACK (5s Timeout)
	// 简单轮询读取
	ackReceived := false
	timeout := time.Now().Add(5 * time.Second)
	buf := make([]byte, 1024)

	for time.Now().Before(timeout) {
		n, err := sm.Port.Read(buf)
		if n > 0 {
			// 检查是否包含 ACK or NAK
			for i := 0; i < n; i++ {
				if buf[i] == protocol.ACK {
					ackReceived = true
					break
				}
				if buf[i] == protocol.NAK {
					return nil, errors.New("received NAK from POS")
				}
			}
		}
		if ackReceived {
			break
		}
		if err != nil && err.Error() != "EOF" {
			// ignore EOF for mock
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !ackReceived {
		return nil, errors.New("timeout waiting for ACK")
	}

	// 5. 等待 Response (60s Timeout)
	fmt.Println("ACK received. Waiting for POS Response...")

	respBuffer := new(bytes.Buffer)
	txTimeout := time.Now().Add(65 * time.Second)

	for time.Now().Before(txTimeout) {
		n, err := sm.Port.Read(buf)
		if n > 0 {
			respBuffer.Write(buf[:n])

			// 检查是否收到完整包 STX...ETX+LRC
			data := respBuffer.Bytes()
			idxStx := bytes.IndexByte(data, protocol.STX)
			idxEtx := bytes.LastIndexByte(data, protocol.ETX)

			if idxStx >= 0 && idxEtx > idxStx {
				// 确保有 LRC (ETX 后一位)
				if len(data) > idxEtx+1 {
					// 提取完整包 (可能有多余数据在前后? 假设 clear buffer 后 STX 是第一个)
					// 安全起见取 STX 到 LRC
					packetData := data[idxStx : idxEtx+2] // +2 to include LRC

					// 校验
					if protocol.ValidatePacket(packetData) {
						// 回复 ACK
						sm.Port.Write([]byte{protocol.ACK})

						// 解析
						result := protocol.ParseResponse(packetData)
						return result, nil
					} else {
						// 校验失败? 回复 NAK?
						// sm.Port.Write([]byte{protocol.NAK})
						// Continue waiting? or Fail?
						fmt.Println("Received invalid packet checksum")
					}
				}
			}
		}

		if err != nil {
			// handle error
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil, errors.New("transaction timeout")
}
