package protocol

import (
	"bytes"
	"fmt"
	"strings"
)

// Constants
const (
	STX byte = 0x02
	ETX byte = 0x03
	ACK byte = 0x06
	NAK byte = 0x15

	PayloadLen = 600
	FrameLen   = 603
)

type TCBRequest struct {
	TransType string
	Fields    map[string]string
}

// BuildPacket builds a full frame: STX + DATA(600) + ETX + LRC
func BuildPacket(req TCBRequest) ([]byte, error) {
	data, err := BuildPayload(req)
	if err != nil {
		return nil, err
	}

	frame := make([]byte, 0, FrameLen)
	frame = append(frame, STX)
	frame = append(frame, data...)
	frame = append(frame, ETX)
	lrc := CalculateLRC(append(data, ETX))
	frame = append(frame, lrc)
	return frame, nil
}

// BuildPayload builds 600-byte DATA payload from layout
func BuildPayload(req TCBRequest) ([]byte, error) {
	if req.TransType == "" {
		return nil, fmt.Errorf("missing TransType")
	}

	layout, ok, err := GetLayoutByTransType(req.TransType)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no layout for TransType %s", req.TransType)
	}

	data := bytes.Repeat([]byte(" "), PayloadLen)

	fields := map[string]string{}
	for k, v := range req.Fields {
		fields[k] = v
	}
	fields["Trans_Type"] = req.TransType

	for _, f := range layout.Fields {
		val, exists := fields[f.Field]
		if !exists {
			continue
		}
		writeField(data, f.Pos-1, f.Len, val, padTypeForField(f.Field))
	}

	return data, nil
}

func writeField(buf []byte, offset, length int, val string, pad string) {
	if offset < 0 || offset+length > len(buf) {
		return
	}
	if len(val) > length {
		val = val[:length]
	}
	var field string
	if pad == "LEFT_ZERO" {
		field = fmt.Sprintf("%0*s", length, val)
	} else {
		field = fmt.Sprintf("%-*s", length, val)
	}
	copy(buf[offset:offset+length], []byte(field))
}

func padTypeForField(field string) string {
	// Default: string fields are right-padded with spaces
	// Numeric fields are left-padded with zeros
	field = strings.TrimSpace(field)
	if numericFields[field] {
		return "LEFT_ZERO"
	}
	return "RIGHT_SPACE"
}

var numericFields = map[string]bool{
	// IDs / Codes
	"Trans_Type":         true,
	"Host_ID":            true,
	"CardType":           true,
	"Card_Type":          true,
	"Issuer Id":          true,
	"Issuer_ID":          true,
	"IssuerId":           true,
	"Start Get PAN":      true,
	"Only_Credit/CUP":    true,
	"WaveFlag":           true,
	"Order_Status":       true,
	"ProcessStatus":      true,
	"ECR_Response_Code":  true,
	"Host_Response_Code": true,
	"Host_Resp_Code":      true,
	"NP_Resp_Errcode":     true,

	// Amounts
	"Trans_Amount":     true,
	"Auth_Amount":      true,
	"DownPayment":      true,
	"EachPayment":      true,
	"FeeAmt":           true,
	"Redeem_Point_Remain": true,
	"Redeem_Point_Cost":   true,
	"Redeem_ActNum":       true,
	"Redeem_Config":       true,
	"Redeem_Response":     true,
	"Amount_After_Cost":   true,
	"Sale_Amount":         true,
	"Refund_Amount":       true,
	"Sale_Count":          true,
	"Refund_Count":        true,
	"Period":              true,

	// Dates/Times
	"Trans_Date":      true,
	"Trans_Time":      true,
	"Old Trans_Time":  true,
	"Org Trans_Date":  true,
	"Trans_DateTime":  true,
}
