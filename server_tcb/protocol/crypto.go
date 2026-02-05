package protocol

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// CalculateLRC 计算纵向冗余校验
// 规则：Data 所有字节 XOR ETX (STX不参与)
// 输入 data 应当包含 ETX
func CalculateLRC(data []byte) byte {
	var lrc byte = 0
	for _, b := range data {
		lrc ^= b
	}
	return lrc
}

// GenerateCheckMacValue 生成 ECPay 要求的 SHA1 校验码
// rawFields 是按顺序拼接好的字段字符串 (不含 STX/ETX/LRC)
func GenerateCheckMacValue(rawFields string) string {
	hasher := sha1.New()
	hasher.Write([]byte(rawFields))
	return strings.ToUpper(hex.EncodeToString(hasher.Sum(nil)))
}
