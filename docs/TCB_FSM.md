### 1. 状态机设计 (Aligned with PDF v3.6)

本设计严格对齐「合庫 EDC POS 連線規格長度600 Ver 3.6」第 47-50 页，覆盖 ACK/NAK 双发、ACK Lose 处理与重传规则。

#### 1.1 链路层状态机 (Link Layer FSM, POS 视角)

*负责 STX/ETX/LRC 封包、ACK/NAK 双发、重传与 ACK Lose 规则。*

| 状态 | 事件 | 动作 | 下一状态 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| **IDLE** | 上层请求发送 | 组包(加LRC) -> 写入串口 | **WAIT_ACK** | 发送 Request |
| | 收到 `STX` | 重置接收Buffer | **READING** | 收到 EDC Response |
| **WAIT_ACK** | 收到 `ACK` | (无) | **IDLE** | 发送成功 |
| | 收到 `NAK` | 重发次数 < 3 ? 重发 : 报错 | **WAIT_ACK** / **IDLE** | 仅在 NAK 时重发 |
| | 超时 1s | 视为 `ACK` | **IDLE** | **EDC Ack Lose** (PDF p.49) |
| **READING** | 收到数据字节 | 存入 Buffer | **READING** | 固定长度 600+ETX+LRC |
| | 收满 600+ETX+LRC | 计算 LRC | **VERIFY** | 收到完整包 |
| | 接收长度不足/断流 | 发送 `NAK`x2 | **IDLE** | 视为长度错误 |
| **VERIFY** | LRC/长度正确 | 发送 `ACK`x2 -> 通知应用层 | **IDLE** | 接收成功 |
| | LRC/长度错误 | 发送 `NAK`x2 | **IDLE** | EDC 将重送 (PDF p.49) |

**重复 Response 处理**: 若 POS 未在 1s 内回 ACK/NAK，EDC 可能重送 Response，最久 5s (PDF p.50)。POS 必须容忍重复 Response 并再次回 ACKx2，应用层需做幂等处理。

#### 1.2 应用层状态机 (App Layer FSM)

| 状态 | 事件 | 动作 | 下一状态 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| **APP_IDLE** | 用户发起交易 | 构造 Request -> Link.Send | **APP_WAIT_RESP** | |
| **APP_WAIT_RESP** | Link 收到 Response | 解析 -> 返回结果 | **APP_IDLE** | 正常流程 |
| | 总超时 60s | 交易逾时错误 | **APP_IDLE** | PDF: POS Timeout > 60s |

---

### 2. Go 参考实现 (Aligned with PDF v3.6)

```go
package tcbpos

import (
 "bytes"
 "errors"
 "fmt"
 "io"
 "time"
)

// ==========================================
// 1. Constants & Configuration
// ==========================================

const (
 STX byte = 0x02
 ETX byte = 0x03
 ACK byte = 0x06
 NAK byte = 0x15

 PacketLen    = 603 // STX(1) + Data(600) + ETX(1) + LRC(1)
 PayloadLen   = 600
 AckTimeout   = 1 * time.Second
 AckRepeat    = 2            // ACK/NAK must be sent twice
 MaxRetries   = 3            // retries on NAK only (total 4 sends incl. first)
 TransTimeout = 60 * time.Second
)

// ==========================================
// 2. Link Layer (Transport)
// ==========================================

type TcbLink struct {
 port io.ReadWriter // should support Read/Write with timeout
}

func NewTcbLink(port io.ReadWriter) *TcbLink {
 return &TcbLink{port: port}
}

// CalculateLRC: XOR(Data) ^ ETX (STX not included)
func (link *TcbLink) calculateLRC(payload []byte) byte {
 var lrc byte = 0
 for _, b := range payload {
  lrc ^= b
 }
 lrc ^= ETX
 return lrc
}

func (link *TcbLink) writeAck(b byte) {
 // PDF p.47: ACK/NAK must be sent twice
 link.port.Write([]byte{b, b})
}

func (link *TcbLink) setReadDeadline(d time.Duration) {
 if r, ok := link.port.(interface{ SetReadDeadline(time.Time) error }); ok {
  _ = r.SetReadDeadline(time.Now().Add(d))
 }
}

// SendPacket implements PDF p.48-49
func (link *TcbLink) SendPacket(payload []byte) error {
 if len(payload) != PayloadLen {
  return fmt.Errorf("invalid payload length: %d, expected %d", len(payload), PayloadLen)
 }

 frame := make([]byte, 0, PacketLen)
 frame = append(frame, STX)
 frame = append(frame, payload...)
 frame = append(frame, ETX)
 frame = append(frame, link.calculateLRC(payload))

 for attempt := 0; attempt <= MaxRetries; attempt++ {
  if _, err := link.port.Write(frame); err != nil {
   return err
  }

  // Wait for ACK/NAK within 1s
  if err := link.waitAckOrNak(); err == nil {
   // ACK received or timeout => assume ACK (EDC Ack Lose, PDF p.49)
   return nil
  }
  // err means NAK received -> retry
 }

 return errors.New("send packet failed: max retries exceeded on NAK")
}

func (link *TcbLink) waitAckOrNak() error {
 resp := make([]byte, 1)
 deadline := time.Now().Add(AckTimeout)

 for time.Now().Before(deadline) {
  link.setReadDeadline(time.Until(deadline))
  n, err := link.port.Read(resp)
  if err != nil || n == 0 {
   // Timeout -> assume ACK per PDF p.49
   return nil
  }
  if resp[0] == ACK || resp[0] == NAK {
   first := resp[0]
   // ACK/NAK are sent twice; try to drain the duplicate quickly.
   link.setReadDeadline(10 * time.Millisecond)
   _, _ = link.port.Read(resp)
   if first == ACK {
    return nil
   }
   return errors.New("nak")
  }
  // Ignore garbage and keep waiting within 1s
 }

 // Timeout without valid ACK/NAK
 return nil
}

func (link *TcbLink) ReadPacket() ([]byte, error) {
 buffer := make([]byte, 1)

 // 1. Wait for STX
 for {
  if _, err := link.port.Read(buffer); err != nil {
   return nil, err
  }
  if buffer[0] == STX {
   break
  }
 }

 // 2. Read remaining 602 bytes (Data + ETX + LRC)
 raw := make([]byte, PacketLen-1)
 offset := 0
 for offset < len(raw) {
  n, err := link.port.Read(raw[offset:])
  if err != nil {
   link.writeAck(NAK)
   return nil, err
  }
  offset += n
 }

 payload := raw[:PayloadLen]
 etxReceived := raw[PayloadLen]
 lrcReceived := raw[PayloadLen+1]

 // 3. Verify Packet
 if etxReceived != ETX {
  link.writeAck(NAK)
  return nil, errors.New("protocol error: ETX missing")
 }

 calcLrc := link.calculateLRC(payload)
 if calcLrc != lrcReceived {
  link.writeAck(NAK)
  return nil, fmt.Errorf("checksum error: expected %02X got %02X", calcLrc, lrcReceived)
 }

 // 4. Send ACK twice
 link.writeAck(ACK)
 return payload, nil
}

// ==========================================
// 3. Application Layer (Logic & Parsing)
// ==========================================

type TcbClient struct {
 link *TcbLink
}

// Example: Build a Request
func BuildSaleRequest(amount int) []byte {
 payload := bytes.Repeat([]byte(" "), PayloadLen)
 copy(payload[0:], []byte("01"))
 copy(payload[33:], []byte(fmt.Sprintf("%012d", amount)))
 return payload
}
```

---

### 3. 对齐说明

1. **ACK/NAK 双发**: PDF p.47 明确要求 ACK/NAK 发送两次。
2. **EDC Ack Lose**: POS 送出 Request 后 1s 未收到 ACK/NAK，视为 ACK 成功继续等待 Response (PDF p.49)。
3. **POS Ack Lose**: EDC 送出 Response 后 1s 未收到 ACK/NAK，或总时长超过 5s，EDC 视为 NAK 并可能重送 (PDF p.50)。
4. **重试规则**: 仅在收到 NAK 时重送，重送次数为 3 次 (PDF p.48)。
5. **交易超时**: POS Timeout > 60s，Auth Timeout = 60s (PDF p.49-50)。
