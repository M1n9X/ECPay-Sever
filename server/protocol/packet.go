package protocol

import (
	"bytes"
	"fmt"
	"time"
)

// 常量定义
const (
	STX byte = 0x02
	ETX byte = 0x03
	ACK byte = 0x06
	NAK byte = 0x15

	PacketLen = 600
)

// ECPayRequest 封装业务请求参数
type ECPayRequest struct {
	TransType string // 01:Sale, 02:Refund, 60:Void, 50:Settle, 80:Echo
	HostID    string // 01:CreditCard
	Amount    string // 12 chars, no decimal
	OrderNo   string // 20 chars, for Refund/Void ref
	PosTime   string // 14 chars, YYYYMMDDHHMMSS
}

// BuildPacket 构建符合 ECPay 规范的 600字节 + STX/ETX/LRC 的完整帧
func BuildPacket(req ECPayRequest) []byte {
	// 初始化 600 字节 DATA 区域 (填充空格 0x20)
	data := bytes.Repeat([]byte{0x20}, PacketLen)

	// 辅助写入函数
	writeField := func(offset, length int, val string, padType string) {
		// 截断
		if len(val) > length {
			val = val[:length]
		}
		var fieldBytes []byte
		if padType == "LEFT_ZERO" { // 数字: 左补0
			format := fmt.Sprintf("%%0%ds", length)
			fieldBytes = []byte(fmt.Sprintf(format, val))
		} else { // 字符串: 右补空格
			format := fmt.Sprintf("%%-%ds", length)
			fieldBytes = []byte(fmt.Sprintf(format, val))
		}
		copy(data[offset:], fieldBytes)
	}

	// --- 字段映射 (根据 ECPay 规范) ---
	// 1. TransType (0-2)
	writeField(0, 2, req.TransType, "LEFT_ZERO")
	// 2. HostID (2-4)
	writeField(2, 2, req.HostID, "LEFT_ZERO")
	// 5. CUP Flag (29-31)
	writeField(29, 2, "00", "LEFT_ZERO")

	// 6. Amount (31-43) - 金额 (无小数点)
	if req.Amount != "" {
		writeField(31, 12, req.Amount, "LEFT_ZERO")
	} else {
		writeField(31, 12, "0", "LEFT_ZERO")
	}

	// 13. EC Order No (88-108) - 用于退货/取消的原单号
	if req.OrderNo != "" {
		writeField(88, 20, req.OrderNo, "RIGHT_SPACE")
	}

	// 25. POS Req Time (492-506)
	if req.PosTime == "" {
		req.PosTime = time.Now().Format("20060102150405")
	}
	writeField(492, 14, req.PosTime, "LEFT_ZERO")

	// 26. Request Hash (506-546)
	// 关键: Hash 计算范围是 Field 1 到 Field 24 (Bytes 0 - 492)
	// 也就是不包含 Time 和 Hash 字段本身
	hashPayload := data[0:492]
	hashVal := GenerateCheckMacValue(string(hashPayload))
	writeField(506, 40, hashVal, "RIGHT_SPACE")

	// --- 封装帧 STX + DATA + ETX + LRC ---
	frame := new(bytes.Buffer)
	frame.WriteByte(STX)
	frame.Write(data)
	frame.WriteByte(ETX)

	// 计算 LRC (XOR of DATA + ETX)
	// 注意：这里的 DATA 已经是 600 bytes
	lrcPayload := append(data, ETX)
	lrc := CalculateLRC(lrcPayload)
	frame.WriteByte(lrc)

	return frame.Bytes()
}
