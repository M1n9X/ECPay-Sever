package driver

import (
	"bytes"
	"ecpay-server/protocol"
	"fmt"
	"io"
	"sync"
	"time"
)

// MockPort 模拟串口行为
type MockPort struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	mu       sync.Mutex
	closed   bool
	simDelay time.Duration // 模拟 POS 处理耗时
}

func NewMockPort() *MockPort {
	return &MockPort{
		readBuf:  new(bytes.Buffer),
		writeBuf: new(bytes.Buffer),
		simDelay: 2 * time.Second,
	}
}

func (m *MockPort) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, io.EOF
	}

	if m.readBuf.Len() == 0 {
		return 0, nil
	}
	return m.readBuf.Read(p)
}

func (m *MockPort) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, io.ErrClosedPipe
	}

	// 模拟 POS 接收数据
	// 1. 验证收到的包是否合法
	// 简单验证: 检查 STX/ETX
	if len(p) > 0 {
		// 打印收到的数据 (调试用)
		// fmt.Printf("[MockPOS] Received %d bytes\n", len(p))

		valid := false
		// 这里简化判断，实战中应该累积 buffer
		// 假设一次 Write 就是一个完整包 (在 Manager 里是一次写完的)
		if protocol.ValidatePacket(p) {
			valid = true
		}

		// 异步模拟响应
		go m.simulateResponse(valid, p)
	}

	return len(p), nil
}

func (m *MockPort) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockPort) ResetInputBuffer() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBuf.Reset()
	return nil
}

func (m *MockPort) simulateResponse(validReq bool, reqPacket []byte) {
	// 1. 立即回复 ACK 或 NAK
	time.Sleep(100 * time.Millisecond)
	m.mu.Lock()
	if validReq {
		fmt.Println("[MockPOS] Packet Valid. Sending ACK.")
		m.readBuf.WriteByte(protocol.ACK)
	} else {
		fmt.Println("[MockPOS] Packet Invalid. Sending NAK.")
		m.readBuf.WriteByte(protocol.NAK)
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	// 2. 只有 ACK 后由于是 Mock，我们假设这是 Sale/Refund 请求，需要返回结果
	// 解析请求以便构造对应的响应
	reqData := protocol.ParseResponse(reqPacket) // ParseResponse 也可以解析 Request

	// 模拟 POS 处理时间
	time.Sleep(m.simDelay)

	m.mu.Lock()
	defer m.mu.Unlock()

	fmt.Println("[MockPOS] Processing complete. Sending Response.")

	// 构造响应
	resp := protocol.ECPayRequest{
		TransType: reqData["TransType"],
		HostID:    reqData["HostID"],
		Amount:    reqData["Amount"], // 返回相同金额
		OrderNo:   "MOCK_" + time.Now().Format("150405"),
		PosTime:   time.Now().Format("20060102150405"),
	}

	// 构建完整响应包
	// 补充 ApprovalNo 等 Response 特有字段是在 ParseResponse 里提取的
	// BuildPacket 主要是构建 Request，但 Response 结构类似，只是多了 AuthCode 等
	// 我们这里用 hack 方式修改 BuildPacket 或者手动构建 response
	// 为了简单，我们复用 BuildPacket，但把 OrderNo 字段当作 AuthCode 占位?
	// 不，BuildPacket 是严格按 Request 格式打包的。 Response 格式略有不同 (OrderNo位置不一样?)
	// 检查 Docs:
	// Request: OrderNo at 88 (len 20).
	// Response: OrderNo at 88 (len 20). ApprovalNo at 55 (len 6).
	// BuildPacket 写了 OrderNo at 88.
	// 所以我们需要手动把 ApprovalNo 写入 BuildPacket 生成的 bytes 中

	packet := protocol.BuildPacket(resp)

	// 注入 ApprovalNo (Offset 55, Len 6)
	// BuildPacket 默认那是空格
	copy(packet[1+55:], []byte("123456"))

	// 注入 RespCode "0000" (Offset 61, Len 4)
	copy(packet[1+61:], []byte("0000"))

	// 重新计算 Hash 和 LRC?
	// 修改了 Data 内容 (ApprovalNo, RespCode)，必须重新计算 Hash 和 LRC

	// 提取 Data
	data := packet[1:601]

	// 重新计算 Hash (从 Offset 0 到 492)
	// 注意 ApprovalNo(55) 和 RespCode(61) 都在 Hash 范围内 (0-492)
	hashPayload := data[0:492]
	hashVal := protocol.GenerateCheckMacValue(string(hashPayload))

	// 写入新 Hash (Offset 506)
	copy(data[506:], []byte(fmt.Sprintf("%-40s", hashVal))) // Right Space?
	// BuildPacket 里 writeField 506 是 RIGHT_SPACE.
	// 这里简单处理

	// 重新计算 LRC
	lrcPayload := append(data, protocol.ETX)
	lrc := protocol.CalculateLRC(lrcPayload)

	// 重组 Packet
	finalPacket := new(bytes.Buffer)
	finalPacket.WriteByte(protocol.STX)
	finalPacket.Write(data)
	finalPacket.WriteByte(protocol.ETX)
	finalPacket.WriteByte(lrc)

	m.readBuf.Write(finalPacket.Bytes())
}
