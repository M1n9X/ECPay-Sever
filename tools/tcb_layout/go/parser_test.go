package tcb_layout

import (
 "bytes"
 "path/filepath"
 "testing"
)

func setField(payload []byte, pos, length int, value string) {
 start := pos - 1
 end := start + length
 copy(payload[start:end], bytes.Repeat([]byte(" "), length))
 copy(payload[start:end], []byte(value))
}

func TestParseLayout_3_1_1(t *testing.T) {
 layouts, err := LoadLayouts(filepath.FromSlash("../../../docs/tcb_layout_v36.json"))
 if err != nil {
  t.Fatalf("load layouts: %v", err)
 }
 layout, ok := FindLayout(layouts, "3.1.1")
 if !ok {
  t.Fatalf("layout 3.1.1 not found")
 }
 payload := bytes.Repeat([]byte(" "), 600)
 setField(payload, 1, 2, "01")
 setField(payload, 3, 2, "02")
 setField(payload, 5, 6, "ABC123")
 setField(payload, 34, 12, "000000010000")
 setField(payload, 83, 8, "TID12345")

 parsed, err := Parse(layout, payload)
 if err != nil {
  t.Fatalf("parse: %v", err)
 }
 if parsed["Trans_Type"].Raw != "01" {
  t.Fatalf("Trans_Type raw: %q", parsed["Trans_Type"].Raw)
 }
 if parsed["Host_ID"].Raw != "02" {
  t.Fatalf("Host_ID raw: %q", parsed["Host_ID"].Raw)
 }
 if parsed["Invoice_No"].Trimmed != "ABC123" {
  t.Fatalf("Invoice_No trimmed: %q", parsed["Invoice_No"].Trimmed)
 }
 if parsed["Trans_Amount"].Raw != "000000010000" {
  t.Fatalf("Trans_Amount raw: %q", parsed["Trans_Amount"].Raw)
 }
 if parsed["EDC_Terminal_ID"].Trimmed != "TID12345" {
  t.Fatalf("EDC_Terminal_ID trimmed: %q", parsed["EDC_Terminal_ID"].Trimmed)
 }
}

func TestParseLayout_3_1_14(t *testing.T) {
 layouts, err := LoadLayouts(filepath.FromSlash("../../../docs/tcb_layout_v36.json"))
 if err != nil {
  t.Fatalf("load layouts: %v", err)
 }
 layout, ok := FindLayout(layouts, "3.1.14")
 if !ok {
  t.Fatalf("layout 3.1.14 not found")
 }
 payload := bytes.Repeat([]byte(" "), 600)
 setField(payload, 1, 2, "36")
 setField(payload, 34, 12, "000000000500")
 setField(payload, 58, 20, "ORDER123")
 setField(payload, 78, 1, "1")
 setField(payload, 79, 4, "0000")

 parsed, err := Parse(layout, payload)
 if err != nil {
  t.Fatalf("parse: %v", err)
 }
 if parsed["Trans_Type"].Raw != "36" {
  t.Fatalf("Trans_Type raw: %q", parsed["Trans_Type"].Raw)
 }
 if parsed["Order_Num"].Trimmed != "ORDER123" {
  t.Fatalf("Order_Num trimmed: %q", parsed["Order_Num"].Trimmed)
 }
 if parsed["Order_Status"].Raw != "1" {
  t.Fatalf("Order_Status raw: %q", parsed["Order_Status"].Raw)
 }
 if parsed["ECR_Response_Code"].Raw != "0000" {
  t.Fatalf("ECR_Response_Code raw: %q", parsed["ECR_Response_Code"].Raw)
 }
}
