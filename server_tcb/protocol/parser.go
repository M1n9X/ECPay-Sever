package protocol

import (
	"bytes"
	"fmt"
	"strings"
)

// ParseResponse parses a TCB response packet into field map
func ParseResponse(packet []byte) (map[string]string, error) {
	data, err := extractData(packet)
	if err != nil {
		return nil, err
	}
	transType := strings.TrimSpace(string(data[0:2]))
	if transType == "" {
		return nil, fmt.Errorf("missing Trans_Type")
	}

	layout, ok, err := GetLayoutByTransType(transType)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no layout for TransType %s", transType)
	}

	result := make(map[string]string)
	for _, f := range layout.Fields {
		start := f.Pos - 1
		end := start + f.Len
		if start < 0 || end > len(data) {
			continue
		}
		val := string(bytes.TrimRight(data[start:end], " "))
		result[f.Field] = val
	}
	return result, nil
}

func extractData(packet []byte) ([]byte, error) {
	if len(packet) == FrameLen && packet[0] == STX {
		return packet[1 : 1+PayloadLen], nil
	}
	if len(packet) == PayloadLen {
		return packet, nil
	}
	if len(packet) > PayloadLen {
		return packet[:PayloadLen], nil
	}
	return nil, fmt.Errorf("invalid packet length: %d", len(packet))
}

// ValidatePacket validates STX/ETX/LRC for a full frame
func ValidatePacket(packet []byte) bool {
	if len(packet) != FrameLen {
		return false
	}
	if packet[0] != STX || packet[601] != ETX {
		return false
	}
	payload := packet[1:602]
	recLrc := packet[602]
	calc := CalculateLRC(payload)
	return calc == recLrc
}
