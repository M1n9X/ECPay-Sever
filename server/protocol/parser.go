package protocol

import (
	"bytes"
)

// ParseResponse 解析 ECPay POS 返回的 600 字节 DATA
func ParseResponse(packet []byte) map[string]string {
	// packet 应该已经去除了 STX/ETX/LRC，或者我们只取 DATA 部分
	// 如果传入的是完整帧，我们需要提取 DATA
	// 完整帧: STX(1) + DATA(600) + ETX(1) + LRC(1)
	var data []byte

	if len(packet) == 603 && packet[0] == STX {
		data = packet[1:601]
	} else if len(packet) == 600 {
		data = packet
	} else {
		// 尝试防御性解析，只要长度够 600
		if len(packet) >= 600 {
			data = packet[:600]
		} else {
			return map[string]string{"Error": "Invalid Packet Length"}
		}
	}

	readField := func(offset, length int) string {
		if offset+length > len(data) {
			return ""
		}
		return string(bytes.TrimSpace(data[offset : offset+length]))
	}

	return map[string]string{
		"TransType":  readField(0, 2),
		"HostID":     readField(2, 2),
		"Amount":     readField(31, 12),
		"TransDate":  readField(43, 6),
		"TransTime":  readField(49, 6),
		"ApprovalNo": readField(55, 6),   // 授权码
		"RespCode":   readField(61, 4),   // 0000 = Success
		"TerminalID": readField(65, 8),   // 终端机号
		"MerchantID": readField(73, 15),  // 商店代号
		"OrderNo":    readField(88, 20),  // 绿界单号
		"StoreID":    readField(108, 18), // 柜号
		"CardType":   readField(126, 2),  // 卡片代码: 00=VISA, 01=MC, 02=JCB, 03=CUP
		"CardNo":     readField(10, 19),  // 掩码卡号
	}
}

// ValidatePacket 校验接收到的完整帧是否合法 (LRC 校验)
func ValidatePacket(packet []byte) bool {
	if len(packet) != 603 {
		return false
	}
	if packet[0] != STX {
		return false
	}
	// ETX 应在 601 (Index)
	if packet[601] != ETX {
		return false
	}

	// 校验 LRC
	// 取 DATA + ETX
	payload := packet[1:602]
	recLrc := packet[602]

	calcLrc := CalculateLRC(payload)
	return calcLrc == recLrc
}
