# TCB (Taiwan Cooperative Bank) POS RS232 Specification (Aligned with PDF v3.6)

> Source: 合作金庫 端末機與收銀機連線規格書 長度600Byte Version 3.6
> Status: Mirror of PDF v3.6 (Authoritative)
> Last Updated: 2026-02-04

## 1. Connection Parameters (Physical Layer)

| Parameter | Value | Notes |
| :--- | :--- | :--- |
| Baud Rate | 115200 | Default. Range: 110-115200 |
| Data Bits | 8 | |
| Parity | None | |
| Stop Bits | 1 | |
| Flow Control | Not specified | |

### 1.1 Protocol Timing & Handshake

| Item | Value | Notes |
| :--- | :--- | :--- |
| ACK Timeout | 1 second | Receiver must reply ACK/NAK within 1s |
| ACK/NAK Repeat | 2 times | ACK and NAK are sent twice (2 bytes) |
| Retry Limit | 3 retries | Retransmit only on NAK (total 4 sends incl. first), for both Request and Response |
| Inter-byte Delay | 5ms (optional) | If enabled, delay 5ms per byte |
| Response ACK-Lose Total | 5 seconds | If EDC doesn’t receive ACK/NAK within 1s or total >5s, it assumes NAK and continues |

#### 1.2 ACK Lose Rules (PDF p.49-50)

- EDC Ack Lose (Request side, PDF p.49): POS sends Request; if no ACK/NAK within 1s, POS assumes ACK and continues waiting for Response.
- POS Ack Lose (Response side, PDF p.50): EDC sends Response; if no ACK/NAK within 1s or total >5s, EDC assumes NAK and may resend. POS must tolerate duplicate responses and ACK them again.

### 1.2 Test Program Config (PDF 2.1)

Test tool uses `InData.dat` / `OutData.dat` (600 bytes). If `OutData.dat` returns `C005`/`S008`, check the tool's README and verify EDC settings.

`Config.dat` parameters:

| Item | Description | Default |
| :--- | :--- | :--- |
| Com Port Select | Com1 or Com2 | |
| Timeout Setup (Second) | 幾秒後視為斷線並離開程式 | |
| Baud Rate | 110 to 115200 | 115200 |
| Communication Protocol | Bits (5/6/7/8), Parity (N/E/O), Stop Bits (1/2) | 8N1 |
| Error Times | 錯誤次數上限，超過視為斷線 | 3 |
| Delay Time Enable | 接收一個 byte 延遲 5ms | N |

## 2. Packet Structure (Link Layer)

Fixed Length: 603 bytes (600-byte DATA payload)

| Byte | Length | Name | Value | Description |
| :--- | :--- | :--- | :--- | :--- |
| 1 | 1 | STX | 0x02 | Start of Text |
| 2-601 | 600 | DATA | ASCII | Application payload (see Section 6) |
| 602 | 1 | ETX | 0x03 | End of Text |
| 603 | 1 | LRC | Calc | XOR(DATA[1..600]) XOR ETX (STX not included) |

ACK byte = 0x06, NAK byte = 0x15 (both sent twice).

## 3. Transaction Types (Trans_Type)

Trans_Type is a 2-byte field at DATA position 1.

| Code | Description |
| :--- | :--- |
| 01 | SALE (購貨) |
| 02 | REFUND (退貨) |
| 03 | INST SALE (分期) |
| 04 | INST REFUND (分期退貨) |
| 05 | NATIONAL PAYMENT (全國性繳費) |
| 06 | NORMAL PAYMENT (一般繳費) |
| 07 | INQ TIMEOUT TRANS (逾時交易查詢) |
| 11 | BATCH_RETURN (主機帳務回傳) |
| 20 | SELF-SALE (無人自助交易純讀卡) |
| 21 | NP SELF-SALE (無人全國性繳費自助交易純讀卡) |
| 22 | ALIPAY SALE (支付寶交易) |
| 23 | ALIPAY VOID (支付寶取消) |
| 24 | ALIPAY REFUND (支付寶退貨) |
| 25 | SELF-SALE (無人自助交易) |
| 26 | NP SELF-SALE (無人全國性繳費自助交易) |
| 27 | FISC REFUND (金融卡退貨) |
| 30 | VOID (取消) |
| 31 | CMAS SALE (悠遊卡購貨) |
| 32 | CMAS REFUND (悠遊卡退貨) |
| 36 | INTELLA_OLPAY (英特拉主掃) |
| 37 | INTELLA_MICROPAY (英特拉被掃) |
| 38 | INTELLA_REFUND (英特拉退款) |
| 39 | INTELLA_INQUIRY (英特拉查詢) |
| 50 | SETTLE (結帳) |
| 51 | AUTO SETTLE (自動結帳) |
| 52 | PRINT STATEMENT (列印主機帳務明細) |
| 60 | GET PAN (從刷卡機取得金融卡或信用卡卡號) |
| 62 | SALE (必須接在 60 command 之後) |
| 70 | TERMINATE (在 60 command 之後終止交易) |
| 80 | REDE SALE (紅利) |
| 81 | REDE REFUND (紅利退貨) |
| 91 | TRANS RETURN (交易回傳) |
| 92 | USERLOGON RETURN (使用者登入回傳) |

Kiosk mode notes (PDF p.27-28):
- 一般自助模式: 25 -> 一般信用卡 (3.1.1), 26 -> 一般金融卡 (3.1.1)
- 全繳自助模式: 25 -> 一般信用卡 (3.1.1), 26 -> 全國性繳費 (3.1.3)

## 4. Host_ID (授權中心編碼)

| Code | Bank |
| :--- | :--- |
| 00 | 花旗/大來 |
| 01 | 美國運通 |
| 02 | 合庫 (一般信用卡及紅利) |
| 03 | 財金 |
| 04 | 合庫 (分期付款) |
| 05 | 悠遊卡 |
| 06 | 一卡通 |
| 07 | DCC |
| 08 | 全國性繳費 (僅供台北榮總使用) |
| 99 | 其他 |

## 5. Data Field Rules

- All DATA positions in Section 6 are 1-based and exactly match the PDF.
- For 0-based indexing into the 600-byte DATA array, use `offset = position - 1`.
- Trans_Amount includes 2 decimal digits but no decimal point.
- Trans_Date format: `YYMMDD` (西元年).
- Trans_Time format: `HHmmss` (24-hour).

## 6. DATA Layouts (600 bytes)

### 6.1 3.1.1 一般交易 (含支付寶)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 6 | Invoice_No | EDC 簽單調閱編號 |
| 11 | 19 | Card_No | 信用卡卡號 (左靠右補空白) |
| 30 | 4 | Card_Expire_Date | 信用卡有效期 (全部空白) |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 6 | Trans_Date | 交易日期 |
| 52 | 6 | Trans_Time | 交易時間 |
| 58 | 9 | Approval_No | 授權碼 (左靠右補空白) |
| 67 | 12 | Auth_Amount | 預先授權金額 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 12 | Reference_No | 銀行交易序號 |
| 103 | 12 | Exp_Amount | 其他金額 |
| 115 | 18 | Store_Id | 專櫃號 |
| 133 | 2 | Start Get PAN | 交易別 (SALE 01, REFUND 02) |
| 135 | 3 | Issuer Id | 發卡代號 |
| 138 | 1 | CardType | 卡片代碼 |
| 139 | 4 | Filler | 保留 |
| 143 | 1 | Only_Credit/CUP | 強制信用卡/銀聯卡 |
| 144 | 1 | Filler | 保留 |
| 145 | 50 | Encrypted Card Number | 加密卡號 |
| 195 | 16 | Cancel Debt Number | 銷帳編號 |
| 211 | 6 | STAN | 端末交易序號 (支付寶) |
| 217 | 16 | Order_Num | 訂單編號 (支付寶) |
| 233 | 16 | Old Order_Num | 原訂單編號 (支付寶) |
| 249 | 14 | Old Trans_Time | 原交易時間 (支付寶) |
| 263 | 32 | ALIPAY_ID | 支付寶條碼 |
| 295 | 4 | Host_Response_Code | 信用卡主機回應碼 |
| 299 | 15 | EDC_Merchant_ID | EDC 商店代號 |
| 314 | 5 | Filler | 保留 |
| 319 | 30 | Additional Information | 附加資料 (左靠右補空白) |
| 349 | 1 | ESC_Status | 電簽簽名狀態 |
| 350 | 4 | ESC_Response_Code | 電簽主機回應碼 |
| 354 | 247 | Filler | 保留 |

Notes:
- Encrypted Card Number only returned when EDC e-invoice carrier is enabled.
- Alipay fields only used for Alipay transactions.
- Only_Credit/CUP: "2" forces CUP; "1" forces credit card.
- ESC_Status and ESC_Response_Code only returned when ECR e-sign upload switch is enabled.

### 6.2 3.1.2 分期交易

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 6 | Invoice_No | EDC 簽單調閱編號 |
| 11 | 19 | Card_No | 信用卡卡號 (左靠右補空白) |
| 30 | 4 | Card_Expire_Date | 信用卡有效期 (全部空白) |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 6 | Trans_Date | 交易日期 |
| 52 | 6 | Trans_Time | 交易時間 |
| 58 | 9 | Approval_No | 授權碼 (左靠右補空白) |
| 67 | 12 | DownPayment | 首期金額 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 12 | Reference_No | 銀行交易序號 |
| 103 | 12 | EachPayment | 每期金額 |
| 115 | 18 | Store_Id | 專櫃號 |
| 133 | 2 | Start Get PAN | 交易別 (分期 SALE 03, 分期退貨 04) |
| 135 | 2 | Period | 分期期數 |
| 137 | 1 | Filler | 保留 |
| 138 | 1 | CardType | 卡片代碼 |
| 139 | 6 | InterestAmt | 手續費 |
| 145 | 50 | Encrypted Card Number | 加密卡號 |
| 195 | 16 | Cancel Debt Number | 銷帳編號 |
| 211 | 108 | Filler | 保留 |
| 319 | 30 | Additional Information | 附加資料 (左靠右補空白) |
| 349 | 1 | ESC_Status | 電簽簽名狀態 |
| 350 | 4 | ESC_Response_Code | 電簽主機回應碼 |
| 354 | 247 | Filler | 保留 |

Notes:
- ESC_Status and ESC_Response_Code only returned when ECR e-sign upload switch is enabled.

### 6.3 3.1.3 繳費交易 (含全國性繳費)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 6 | Invoice_No | EDC 簽單調閱編號 |
| 11 | 19 | Card_No | 金融卡帳號 (左靠右補空白) |
| 30 | 4 | IssureReturnCode | 主機回應碼 |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 6 | Trans_Date | 交易日期 |
| 52 | 6 | Trans_Time | 交易時間 |
| 58 | 7 | IssureSeqNo | 主機查詢序號 |
| 65 | 4 | FeeAmt | 繳費作業手續費 |
| 69 | 6 | ProcessCode | 處理代碼 (逾時查詢用) |
| 75 | 4 | FiscCode | 交易類別代碼 (逾時查詢用) |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 3 | ProcessStatus | 主機處理狀況 (逾時查詢用) |
| 94 | 21 | Filler | 保留 |
| 115 | 18 | Store_Id | 專櫃號 |
| 133 | 2 | Start Get PAN | 交易別 (全國繳費 05, 一般繳費 06) |
| 135 | 3 | Issuer Id | 發卡代號 |
| 138 | 1 | CardType | 卡片代碼 |
| 139 | 6 | Filler | 保留 |
| 145 | 50 | Encrypted Card Number | 加密卡號 |
| 195 | 16 | Cancel Debt Number | 銷帳編號 |
| 211 | 4 | Host_Resp_Code | 主機回應碼 (欄位 39) |
| 215 | 4 | NP_Resp_Errcode | 全國性繳費主機回應碼 (欄位 57) |
| 219 | 15 | EDC_Merchant_ID | EDC 商店代號 |
| 234 | 16 | NP_Order_Number | 全繳訂單編號 |
| 250 | 69 | Filler | 保留 |
| 319 | 30 | Additional Information | 附加資料 (左靠右補空白) |
| 349 | 1 | ESC_Status | 電簽簽名狀態 |
| 350 | 4 | ESC_Response_Code | 電簽主機回應碼 |
| 354 | 247 | Filler | 保留 |

Notes:
- 交易別 26 使用此欄位格式。
- 若通訊回應碼回應 0013，會多攜帶訂單編號欄位以利逾時交易查詢。
- ESC_Status and ESC_Response_Code only returned when ECR e-sign upload switch is enabled.

### 6.4 3.1.4 無人信用卡自助交易 (純讀卡，交易別 20)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 6 | Invoice_No | EDC 簽單調閱編號 |
| 11 | 19 | Card_No | 信用卡卡號 (左靠右補空白，不遮掩) |
| 30 | 4 | Card_Expire_Date | 信用卡有效期 (不遮掩) |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 6 | Trans_Date | 交易日期 |
| 52 | 6 | Trans_Time | 交易時間 |
| 58 | 9 | Approval_No | 授權碼 (左靠右補空白) |
| 67 | 12 | Auth_Amount | 預先授權金額 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 12 | Reference_No | 銀行交易序號 |
| 103 | 12 | Exp_Amount | 其他金額 |
| 115 | 18 | Store_Id | 專櫃號 |
| 133 | 2 | Start Get PAN | 交易別 (SALE 01, REFUND 02) |
| 135 | 3 | Issuer Id | 發卡代號 |
| 138 | 1 | CardType | 卡片代碼 |
| 139 | 6 | Filler | 保留 |
| 145 | 50 | Encrypted Card Number | 加密卡號 |
| 195 | 16 | Cancel Debt Number | 銷帳編號 |
| 211 | 6 | STAN | 端末交易序號 (支付寶) |
| 217 | 16 | Order_Num | 訂單編號 (支付寶) |
| 233 | 16 | Old Order_Num | 原訂單編號 (支付寶) |
| 249 | 14 | Old Trans_Time | 原交易時間 (支付寶) |
| 263 | 32 | ALIPAY_ID | 支付寶條碼 (支付寶) |
| 295 | 1 | WaveFlag | 感應交易碼 (1 感應, 0 一般) |
| 296 | 23 | Filler | 保留 |
| 319 | 30 | Additional Information | 附加資料 (左靠右補空白) |
| 349 | 252 | Filler | 保留 |

### 6.5 3.1.5 無人全國性繳費自助交易 (純讀卡，交易別 21)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 8 | IssuerId | 轉出銀行代號 |
| 11 | 8 | STAN | 晶片卡交易序號 |
| 19 | 15 | RFU | 保留 |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 14 | Trans_DateTime | 交易日期時間 |
| 60 | 16 | CardAccount | 轉出帳號 |
| 76 | 3 | RFU | 保留 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 30 | IcCardComment | 晶片卡備註 |
| 121 | 8 | TAC | 晶片交易驗證碼 |
| 129 | 8 | TccCode | 端末設備查核碼 |
| 137 | 1 | WaveFlag | 感應交易碼 (1 感應, 0 一般) |
| 138 | 7 | RFU | 保留 |
| 145 | 50 | Encrypted Card Number | 加密卡號 |
| 195 | 16 | Cancel Debt Number | 銷帳編號 |
| 211 | 108 | Filler | 保留 |
| 319 | 30 | Additional Information | 附加資料 (左靠右補空白) |
| 349 | 252 | Filler | 保留 |

### 6.6 3.1.6 結帳交易 (交易別 50)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 74 | Filler | 保留 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 54 | Filler | 保留 |
| 145 | 12 | Sale_Amount | 銷售總金額 |
| 157 | 12 | Refund_Amount | 退貨總金額 |
| 169 | 3 | Sale_Count | 銷貨筆數 |
| 172 | 3 | Refund_Count | 退貨筆數 |
| 175 | 4 | Host_Resp_Code | 主機回應碼 (欄位 39) |
| 179 | 170 | Filler | 保留 |
| 349 | 1 | ESC_Status | 電簽簽名狀態 |
| 350 | 4 | ESC_Response_Code | 電簽主機回應碼 |
| 354 | 247 | Filler | 保留 |

Notes:
- 全國性繳費無退貨功能，退貨欄位改帶一般繳費的金額與筆數。
- ESC_Status and ESC_Response_Code only returned when ECR e-sign upload switch is enabled.

### 6.7 3.1.7 連動結帳交易 (交易別 51)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 74 | Filler | 保留 |
| 79 | 4 | TCB_ECR_Response_Code | 收銀機通訊回應碼 (TCB) |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 1 | TCB_SETTLE_ESC_Status | 信用卡結帳電簽簽名狀態 |
| 92 | 4 | TCB_SETTLE_ESC_Response_Code | 信用卡結帳電簽主機回應碼 |
| 96 | 1 | FISC_SETTLE_ESC_Status | 金融卡結帳電簽簽名狀態 |
| 97 | 4 | FISC_SETTLE_ESC_Response_Code | 金融卡結帳電簽主機回應碼 |
| 101 | 1 | NP_SETTLE_ESC_Status | 全國性繳費結帳電簽簽名狀態 |
| 102 | 4 | NP_SETTLE_ESC_Response_Code | 全國性繳費結帳電簽主機回應碼 |
| 106 | 1 | AE_SETTLE_ESC_Status | 美國運通結帳電簽簽名狀態 |
| 107 | 4 | AE_SETTLE_ESC_Response_Code | 美國運通結帳電簽主機回應碼 |
| 111 | 34 | Filler | 保留 |
| 145 | 12 | TCB_Sale_Amount | 信用卡銷售總金額 |
| 157 | 12 | TCB_Refund_Amount | 信用卡退貨總金額 |
| 169 | 3 | TCB_Sale_Count | 信用卡銷貨筆數 |
| 172 | 3 | TCB_Refund_Count | 信用卡退貨筆數 |
| 175 | 4 | TCB_Host_Resp_Code | 信用卡主機回應碼 |
| 179 | 4 | FISC_ECR_Response_Code | 收銀機通訊回應碼 (FISC) |
| 183 | 12 | FISC_Sale_Amount | 金融卡銷售總金額 |
| 195 | 12 | FISC_Refund_Amount | 金融卡退貨總金額 |
| 207 | 3 | FISC_Sale_Count | 金融卡銷貨筆數 |
| 210 | 3 | FISC_Refund_Count | 金融卡退貨筆數 |
| 213 | 4 | FISC_Host_Resp_Code | 金融卡主機回應碼 |
| 217 | 4 | NP_ECR_Response_Code | 收銀機通訊回應碼 (NP) |
| 221 | 12 | NP_Sale_Amount | 全國性繳費銷售總金額 |
| 233 | 3 | NP_Sale_Count | 全國性繳費銷貨筆數 |
| 236 | 4 | NP_Host_Resp_Code_F39 | 全國性繳費主機回應碼 (F39) |
| 240 | 4 | NP_Host_Resp_Code_F57 | 全國性繳費主機回應碼 (F57) |
| 244 | 4 | INST_ECR_Response_Code | 收銀機通訊回應碼 (INST) |
| 248 | 12 | INST_Sale_Amount | 分期銷售總金額 |
| 260 | 12 | INST_Refund_Amount | 分期退貨總金額 |
| 272 | 3 | INST_Sale_Count | 分期銷貨筆數 |
| 275 | 3 | INST_Refund_Count | 分期退貨筆數 |
| 278 | 4 | INST_Host_Resp_Code | 分期主機回應碼 |
| 282 | 4 | AE_ECR_Response_Code | 收銀機通訊回應碼 (AE) |
| 286 | 12 | AE_Sale_Amount | 美國運通銷售總金額 |
| 298 | 12 | AE_Refund_Amount | 美國運通退貨總金額 |
| 310 | 3 | AE_Sale_Count | 美國運通銷貨筆數 |
| 313 | 3 | AE_Refund_Count | 美國運通退貨筆數 |
| 316 | 4 | AE_Host_Resp_Code | 美國運通主機回應碼 |
| 320 | 4 | CMAS_ECR_Response_Code | 收銀機通訊回應碼 (CMAS) |
| 324 | 12 | CMAS_Sale_Amount | 悠遊卡銷售總金額 |
| 336 | 12 | CMAS_Refund_Amount | 悠遊卡退貨總金額 |
| 348 | 3 | CMAS_Sale_Count | 悠遊卡銷貨筆數 |
| 351 | 3 | CMAS_Refund_Count | 悠遊卡退貨筆數 |
| 354 | 4 | CMAS_Host_Resp_Code | 悠遊卡主機回應碼 |
| 358 | 243 | Filler | 保留 |

Notes:
- 僅回傳 EDC 已開啟之主機欄位，未開啟者回空值。
- 全國性繳費無退貨功能，故不回傳退貨筆數及金額。
- 若連動結帳有主機結帳失敗，回傳對應主機通訊錯誤碼且不帶帳務資料。
- 此處金融卡為一般商店金融卡交易，與全國性繳費交易為不同交易類型。
- 電簽簽名狀態與回應碼需 ECR 電簽上傳開關。

### 6.8 3.1.8 主機帳務回傳 (交易別 11)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | DINES | 大來卡 (代號 00) |
| 5 | 12 | DINES_Sale_Amount | 銷售總金額 (含小費) |
| 17 | 12 | DINES_Refund_Amount | 退貨總金額 |
| 29 | 12 | DINES_Void_Amount | 取消總金額 |
| 41 | 3 | DINES_Sale_Count | 銷貨筆數 |
| 44 | 3 | DINES_Refund_Count | 退貨筆數 |
| 47 | 3 | DINES_Void_Count | 取消筆數 |
| 50 | 2 | AE | 美國運通卡 (代號 01) |
| 52 | 12 | AE_Sale_Amount | 銷售總金額 (含小費) |
| 64 | 12 | AE_Refund_Amount | 退貨總金額 |
| 76 | 12 | AE_Void_Amount | 取消總金額 |
| 88 | 3 | AE_Sale_Count | 銷貨筆數 |
| 91 | 3 | AE_Refund_Count | 退貨筆數 |
| 94 | 3 | AE_Void_Count | 取消筆數 |
| 97 | 2 | TCB | 合庫 (代號 02) |
| 99 | 12 | TCB_Sale_Amount | 銷售總金額 (含小費) |
| 111 | 12 | TCB_Refund_Amount | 退貨總金額 |
| 123 | 12 | TCB_Void_Amount | 取消總金額 |
| 135 | 3 | TCB_Sale_Count | 銷貨筆數 |
| 138 | 3 | TCB_Refund_Count | 退貨筆數 |
| 141 | 3 | TCB_Void_Count | 取消筆數 |
| 144 | 2 | FISC | 金融卡 (代號 03) |
| 146 | 12 | FISC_Sale_Amount | 銷售總金額 (含小費) |
| 158 | 12 | FISC_Refund_Amount | 退貨總金額 |
| 170 | 12 | FISC_Void_Amount | 取消總金額 |
| 182 | 3 | FISC_Sale_Count | 銷貨筆數 |
| 185 | 3 | FISC_Refund_Count | 退貨筆數 |
| 188 | 3 | FISC_Void_Count | 取消筆數 |
| 191 | 2 | INST | 分期主機 (代號 04) |
| 193 | 12 | INST_Sale_Amount | 銷售總金額 (含小費) |
| 205 | 12 | INST_Refund_Amount | 退貨總金額 |
| 217 | 12 | INST_Void_Amount | 取消總金額 |
| 229 | 3 | INST_Sale_Count | 銷貨筆數 |
| 232 | 3 | INST_Refund_Count | 退貨筆數 |
| 235 | 3 | INST_Void_Count | 取消筆數 |
| 238 | 363 | Filler | 保留 |

Notes:
- 取消筆數與金額僅提供特定商店使用；非該模式時銷貨與取消會相加減。

### 6.9 3.1.9 紅利交易 (交易別 80, 81)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 6 | Invoice_No | EDC 簽單調閱編號 |
| 11 | 19 | Card_No | 信用卡卡號 (左靠右補空白) |
| 30 | 4 | Card_Expire_Date | 信用卡有效期 (全部空白) |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 6 | Trans_Date | 交易日期 |
| 52 | 6 | Trans_Time | 交易時間 |
| 58 | 9 | Approval_No | 授權碼 (左靠右補空白) |
| 67 | 6 | Redeem_ActNum | 紅利活動編號 |
| 73 | 1 | Redeem_Config | 紅利折抵說明 |
| 74 | 2 | Redeem_Response | 紅利折抵結果回應碼 |
| 76 | 1 | Redeem_Balance_Sign | 紅利餘額點數正負號 |
| 77 | 2 | Filler | 保留 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 12 | Reference_No | 銀行交易序號 |
| 103 | 12 | Amount_After_Cost | 扣抵後消費金額 |
| 115 | 18 | Store_Id | 專櫃號 |
| 133 | 2 | Start Get PAN | 交易別 |
| 135 | 3 | Filler | 保留 |
| 138 | 1 | CardType | 卡片代碼 |
| 139 | 4 | Org_Trans_Date | 原交易日期 (MMDD) |
| 143 | 2 | Filler | 保留 |
| 145 | 50 | Encrypted Card Number | 加密卡號 |
| 195 | 10 | Redeem_Point_Remain | 紅利餘額點數 (右靠左補 0) |
| 205 | 10 | Redeem_Point_Cost | 扣抵紅利點數 (右靠左補 0) |
| 215 | 104 | Filler | 保留 |
| 319 | 30 | Additional Information | 附加資料 (左靠右補空白) |
| 349 | 1 | ESC_Status | 電簽簽名狀態 |
| 350 | 4 | ESC_Response_Code | 電簽主機回應碼 |
| 354 | 247 | Filler | 保留 |

Notes:
- ESC_Status and ESC_Response_Code only returned when ECR e-sign upload switch is enabled.

### 6.10 3.1.10 悠遊卡交易 (交易別 31, 32)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 29 | Filler | 保留 |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 6 | Trans_Date | 交易日期 |
| 52 | 6 | Trans_Time | 交易時間 |
| 58 | 21 | Filler | 保留 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 14 | Reference_No | 銀行交易序號 |
| 103 | 234 | Filler | 保留 |
| 337 | 1 | Ticket_Type | 電子票證名稱類別 |
| 338 | 19 | Ticket_Card_Number | 電子票證卡號 |
| 357 | 10 | Ticket_Reference_Number | 電子票證交易序號 |
| 367 | 10 | Ticket_Batch_Number | 批次號碼 |
| 377 | 10 | Ticket_Pre_Balance | 交易前餘額 |
| 387 | 10 | Ticket_Auto_Load_Amount | 自動加值金額 |
| 397 | 10 | Ticket_Balance | 消費後餘額 |
| 407 | 194 | Filler | 保留 |

### 6.11 3.1.11 金融卡退貨 (交易別 27)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 6 | Invoice_No | EDC 簽單調閱編號 |
| 11 | 19 | Card_No | 信用卡卡號 (左靠右補空白) |
| 30 | 4 | Card_Expire_Date | 信用卡有效期 (全部空白) |
| 34 | 12 | Trans_Amount | 交易金額 (小於等於原交易) |
| 46 | 6 | Trans_Date | 原交易日期 |
| 52 | 6 | Trans_Time | 原交易時間 |
| 58 | 9 | Approval_No | 授權碼 (左靠右補空白) |
| 67 | 12 | Auth_Amount | 預先授權金額 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | 原 EDC 端末機代號 |
| 91 | 12 | Reference_No | 原交易序號 |
| 103 | 12 | Exp_Amount | 其他金額 |
| 115 | 18 | Store_Id | 專櫃號 |
| 133 | 2 | Start Get PAN | 交易別 (SALE 01, REFUND 02) |
| 135 | 3 | Issuer Id | 發卡代號 |
| 138 | 1 | CardType | 卡片代碼 |
| 139 | 6 | Batch_Number | 原交易批次 |
| 145 | 50 | Encrypted Card Number | 加密卡號 |
| 195 | 16 | Cancel Debt Number | 銷帳編號 |
| 211 | 84 | Filler | 保留 |
| 295 | 4 | Host_Response_Code | 金融卡主機回應碼 |
| 299 | 15 | EDC_Merchant_ID | 商代 |
| 314 | 5 | Filler | 保留 |
| 319 | 30 | Additional Information | 附加資料 (左靠右補空白) |
| 349 | 1 | ESC_Status | 電簽簽名狀態 |
| 350 | 4 | ESC_Response_Code | 電簽主機回應碼 |
| 354 | 247 | Filler | 保留 |

Notes:
- 金融卡退貨需帶入交易金額、原交易序號，且金額須小於等於原交易金額。
- 同筆交易序號只能退一次，否則會被財金主機拒絕。
- ESC_Status and ESC_Response_Code only returned when ECR e-sign upload switch is enabled.

### 6.12 3.1.12 交易回傳 (交易別 91)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 6 | Invoice_No | EDC 簽單調閱編號 |
| 11 | 19 | Card_No | 信用卡卡號 (左靠右補空白) |
| 30 | 4 | Card_Expire_Date | 信用卡有效期 (全部空白) |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 6 | Trans_Date | 交易日期 |
| 52 | 6 | Trans_Time | 交易時間 |
| 58 | 9 | Approval_No | 授權碼 (左靠右補空白) |
| 67 | 12 | Auth_Amount | 預先授權金額 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 12 | Reference_No | 銀行交易序號 |
| 103 | 12 | Exp_Amount | 其他金額 |
| 115 | 18 | Store_ID | 專櫃號 |
| 133 | 2 | Start Get PAN | 交易別 (SALE 01, REFUND 02) |
| 135 | 3 | Issuer_ID | 發卡代號 |
| 138 | 1 | Card_Type | 卡片代碼 |
| 139 | 4 | Org_Trans_Date | 原交易日期 (MMDD) |
| 143 | 2 | Filler | 保留 |
| 145 | 50 | Encrypted Card Number | 加密卡號 |
| 195 | 16 | Cancel Debt Number | 銷帳編號 |
| 211 | 6 | STAN | 端末交易序號 (支付寶) |
| 217 | 16 | Order_Num | 訂單編號 (支付寶) |
| 233 | 16 | Old Order_Num | 原訂單編號 (支付寶) |
| 249 | 14 | Old Trans_Time | 原交易時間 (支付寶) |
| 263 | 32 | ALIPAY_ID | 支付寶條碼 |
| 295 | 4 | Host_Response_Code | 信用卡主機回應碼 |
| 299 | 15 | EDC_Merchant_ID | EDC 商店代號 |
| 314 | 287 | Filler | 保留 |

Notes:
- 回傳資料需依原交易類型回傳授權結果。
- Trans_Type Response 請回傳原交易類別。
- 若收銀機未給 Invoice_No，回傳最後一筆。

### 6.13 3.1.13 使用者登入回傳 (交易別 92)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 2 | Host_ID | 銀行別 |
| 5 | 6 | Invoice_No | EDC 簽單調閱編號 |
| 11 | 19 | Card_No | 信用卡卡號 (左靠右補空白) |
| 30 | 4 | Card_Expire_Date | 信用卡有效期 (全部空白) |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 6 | Trans_Date | 交易日期 |
| 52 | 6 | Trans_Time | 交易時間 |
| 58 | 9 | Approval_No | 授權碼 (左靠右補空白) |
| 67 | 12 | Auth_Amount | 預先授權金額 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 8 | EDC_Terminal_ID | EDC 端末機代號 |
| 91 | 12 | Reference_No | 銀行交易序號 |
| 103 | 12 | Exp_Amount | 其他金額 |
| 115 | 18 | Store_Id | 專櫃號 |
| 133 | 2 | Start Get PAN | 交易別 (SALE 01, REFUND 02) |
| 135 | 3 | Issuer Id | 發卡代號 |
| 138 | 1 | CardType | 卡片代碼 |
| 139 | 6 | Filler | 保留 |
| 145 | 50 | Encrypted Card Number | 加密卡號 |
| 195 | 16 | Cancel Debt Number | 銷帳編號 |
| 211 | 6 | STAN | 端末交易序號 (支付寶) |
| 217 | 16 | Order_Num | 訂單編號 (支付寶) |
| 233 | 16 | Old Order_Num | 原訂單編號 (支付寶) |
| 249 | 14 | Old Trans_Time | 原交易時間 (支付寶) |
| 263 | 32 | ALIPAY_ID | 支付寶條碼 |
| 295 | 4 | Host_Response_Code | 信用卡主機回應碼 |
| 299 | 15 | EDC_Merchant_ID | EDC 商店代號 |
| 314 | 287 | Filler | 保留 |

Notes:
- ECR_Response_Code 0000 表示使用者登入成功，0001 表示失敗。

### 6.14 3.1.14 英特拉交易 (交易別 36, 37, 38, 39)

| Pos | Len | Field | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2 | Trans_Type | 交易別 |
| 3 | 31 | Filler | 保留 |
| 34 | 12 | Trans_Amount | 交易金額 |
| 46 | 6 | Trans_Date | 交易日期 |
| 52 | 6 | Trans_Time | 交易時間 |
| 58 | 20 | Order_Num | 訂單編號 |
| 78 | 1 | Order_Status | 訂單狀態 |
| 79 | 4 | ECR_Response_Code | 通訊回應碼 |
| 83 | 4 | INTELLA_Response_Code | 英特拉回應碼 |
| 87 | 512 | Scan_Data | 英特拉被掃掃碼資料 (左靠右補空白) |
| 599 | 2 | Filler | 保留 |

Notes:
- 交易金額除查詢交易外，其餘交易為必帶值。
- 訂單編號僅退款、查詢交易需要帶入，若未帶入則由卡機輸入。
- Scan_Data 僅被掃交易需要帶入，若未帶入則由卡機掃碼。
- 訂單狀態僅查詢交易與主掃交易才會回傳。

## 7. Field Definitions (PDF 3.6)

### 7.1 Start Get PAN
啟動 START 交易時，後續交易之交易別 (SALE or REFUND)。

### 7.2 ECR_Response_Code

| Code | Description |
| :--- | :--- |
| 0000 | 授權 |
| 0001 | 拒絕 |
| 0002 | 請查詢銀行 |
| 0003 | 操作逾時 |
| 0004 | 操作錯誤 |
| 0005 | 通訊失敗 |
| 0006 | 使用者終止交易 |
| 0007 | 終止查詢號碼 70 |
| 0008 | 檢核 TID 不符 |
| 0009 | 使用卡別錯誤 |
| 0010 | 卡片密碼錯誤 |
| 0011 | 連線逾時 |
| 0012 | 卡機未結帳，請先結帳 |
| 0013 | 全國性繳費交易逾時，請於臨櫃查詢交易結果 |
| 0014 | 交易回傳查無此筆交易紀錄 |

Notes:
- 0005: 與主機建立連線時失敗，為通訊失敗。
- 0011: 與主機建立連線後收送未收到對應資料，為連線逾時。
- 0013: 全國性繳費主機轉送資料時發生交易逾時，需進行逾時交易查詢，卡機端會多回傳全繳訂單編號等資料。

### 7.3 CardType

| Code | Description |
| :--- | :--- |
| 1 | VISA |
| 2 | MASTER |
| 3 | JCB |
| 4 | U_CARD |
| 5 | DINER |
| 6 | AE |
| 7 | SMART CARD |
| 8 | CUP |
| 9 | 其他 |

### 7.4 Encrypted Card Number
長度 50。卡號經 SHA-256 後 (32 碼)，再轉 Base64 (44 碼)，冠上主機回傳值 6 碼，組合成 50 碼 (6+44)。

### 7.5 Cancel Debt Number
未使用請留空白；若使用，必須填滿 16 碼非空白。

### 7.6 支付寶交易欄位
- 取消付款交易之訂單編號為該取消付款交易之訂單編號，其餘交易類推。  
- 原訂單編號用於支付寶取消付款交易與退款交易，為必要欄位。  
- 原交易時間僅用於退款交易，為必要欄位。  
- 支付寶條碼欄位若不滿 32 bytes，需補空白。

### 7.7 Issuer ID
僅供金融卡/全國性繳費相關交易使用。

### 7.8 DCC 交易
若端末機開啟 DCC 功能，交易前會檢查 DCC 參數是否過期；若過期將提示下載 DCC 參數，下載需數分鐘後才可重新交易。

### 7.9 Additional Information
長度 30。收到該欄位有資料後，回傳相同資料給 ECR。

### 7.10 備註
a. 交易別 52 Print Statement 於 V3 客製化參數 001(台北榮總)時僅列印明細交易。  
b. 交易別 50 SETTLE、52 PRINT_STATEMENT、91 TRANS RETURN 於 VX520 客製化參數 020(台北榮總)時，全國性繳費相關操作 HOST_ID 帶 08。  
c. 交易別 50 SETTLE、52 PRINT_STATEMENT、91 TRANS RETURN 於 V3 客製化參數 001(台北榮總)時，全國性繳費相關操作 HOST_ID 帶 08。

### 7.11 Only_Credit/CUP
- 若需強制進行銀聯交易，收銀機需帶 Only_Credit/CUP = \"2\"。  
- 若需強制進行信用卡交易，收銀機需帶 Only_Credit/CUP = \"1\"。  
- 可用於交易別 01、25，用於解決無人自助銀聯雙幣卡問題。

| Code | Meaning |
| :--- | :--- |
| 1 | 信用卡 |
| 2 | 銀聯卡 |

### 7.12 英特拉交易注意事項
- 退款交易回傳訂單編號為該退款交易訂單編號，其餘交易類推。  
- 訂單編號僅退款、查詢交易為必要欄位；若未帶入可在卡機輸入。  
- 訂單狀態僅查詢交易與主掃交易查詢後回傳。  
- 被掃交易 Scan_Data 欄位若不滿 512 bytes，需補空白。  
- 英特拉回應碼請參考 Scan2Pay 交易結果代碼表。  
- 交易成功需同時判斷 ECR_Response_Code 與 INTELLA_Response_Code；若為主掃及查詢需額外判斷 Order_Status。

Order_Status:
| Code | Status |
| :--- | :--- |
| 0 | 處理中 |
| 1 | 交易成功 |
| 2 | 交易失敗 |
| 3 | 退款成功 |
| 4 | 退款失敗 |
| 5 | 訂單資料不存在 |

## 8. 交易欄位 (PDF Section 4)

### 8.1 Sale (銷售) 一段式連線
POS -> EDC: Trans_Type ("01") + Trans_Amount  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + ECR_Response_Code + EDC_Terminal_ID + Reference_No + CardType + Encrypted_Card_Number + CancelDebtNumber + Host_Response_Code + EDC_Merchant_ID + ESC_Status + ESC_Response_Code

Notes:
- Only_Credit/CUP 可用於交易別 01、25，詳見 7.11。  
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功時回傳。

### 8.2 Sale (銷售) 二段式連線
Step A (GET PAN):
POS -> EDC: Trans_Type ("60") + Trans_Amount + Start Get PAN ("01")  
EDC -> POS: Trans_Type + Host_ID + Card_No + Trans_Amount + Trans_Date + Trans_Time + ECR_Response_Code + EDC_Terminal_ID + CardType + EDC_Merchant_ID

Step B (SALE):
POS -> EDC: Trans_Type ("62") + Trans_Amount  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + ECR_Response_Code + EDC_Terminal_ID + Reference_No + CardType + Encrypted_Card_Number + CancelDebtNumber + Host_Response_Code + EDC_Merchant_ID + ESC_Status + ESC_Response_Code

Notes:
- Trans_Type 62 的 Trans_Amount 不可大於 Trans_Type 60 的 Trans_Amount。  
- Only_Credit/CUP 規則同 8.1。  
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.3 Refund (退貨) 一段式連線
POS -> EDC: Trans_Type ("02") + Trans_Amount  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + ECR_Response_Code + EDC_Terminal_ID + Reference_No + CardType + Encrypted_Card_Number + CancelDebtNumber + Host_Response_Code + EDC_Merchant_ID + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.4 Refund (退貨) 二段式連線
Step A (GET PAN):
POS -> EDC: Trans_Type ("60") + Trans_Amount + Start Get PAN ("02")  
EDC -> POS: Trans_Type + Host_ID + Card_No + Trans_Amount + Trans_Date + Trans_Time + ECR_Response_Code + EDC_Terminal_ID + CardType + EDC_Merchant_ID

Step B (REFUND):
POS -> EDC: Trans_Type ("62") + Trans_Amount  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + ECR_Response_Code + EDC_Terminal_ID + Reference_No + CardType + Encrypted_Card_Number + CancelDebtNumber + Host_Response_Code + EDC_Merchant_ID + ESC_Status + ESC_Response_Code

Notes:
- Trans_Type 62 的 Trans_Amount 不可大於 Trans_Type 60 的 Trans_Amount。  
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.5 Sale (分期購貨) 一段式連線
POS -> EDC: Trans_Type ("03") + Trans_Amount + Period  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + DownPayment + ECR_Response_Code + EDC_Terminal_ID + Reference_No + EachPayment + Period + CardType + InsterestAmt + Encrypted_Card_Number + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.6 Sale (分期購貨) 二段式連線
Step A (GET PAN):
POS -> EDC: Trans_Type ("60") + Trans_Amount + Period + Start Get PAN ("03")  
EDC -> POS: Trans_Type + Host_ID + Card_No + Trans_Amount + Trans_Date + Trans_Time + ECR_Response_Code + EDC_Terminal_ID + CardType

Step B (INST SALE):
POS -> EDC: Trans_Type ("62") + Trans_Amount  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + DownPayment + ECR_Response_Code + EDC_Terminal_ID + Reference_No + EachPayment + Period + CardType + InsterestAmt + Encrypted_Card_Number + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.7 Inst Refund (分期退貨) 一段式連線
POS -> EDC: Trans_Type ("02") + Trans_Amount + Reference_No  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + DownPayment + ECR_Response_Code + EDC_Terminal_ID + Reference_No + EachPayment + Period + CardType + InsterestAmt + Encrypted_Card_Number + ESC_Status + ESC_Response_Code

Notes:
- PDF 此處標示 Trans_Type 為 02，與 3.2 Trans_Type 列表中的分期退貨 04 不一致。請實作前與銀行確認。  
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.8 Inst Refund (分期退貨) 二段式連線
Step A (GET PAN):
POS -> EDC: Trans_Type ("60") + Trans_Amount + Reference_No + Start Get PAN ("04")  
EDC -> POS: Trans_Type + Host_ID + Card_No + Trans_Amount + Trans_Date + Trans_Time + ECR_Response_Code + EDC_Terminal_ID + CardType

Step B (INST REFUND):
POS -> EDC: Trans_Type ("62") + Trans_Amount  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + DownPayment + ECR_Response_Code + EDC_Terminal_ID + Reference_No + EachPayment + Period + CardType + InsterestAmt + Encrypted_Card_Number + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.9 Void (一般取消 / 金融卡取消)
POS -> EDC: Trans_Type ("30") + Host_ID + Invoice_No  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + ECR_Response_Code + EDC_Terminal_ID + Reference_No + CardType + Host_Response_Code + EDC_Merchant_ID + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.10 Void (分期取消)
POS -> EDC: Trans_Type ("30") + Host_ID + Invoice_No  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + DownPayment + ECR_Response_Code + EDC_Terminal_ID + Reference_No + EachPayment + Period + CardType + InsterestAmt + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.11 Settle (一般結帳: 信用卡/AE/分期/金融卡)
POS -> EDC: Trans_Type ("50") + Host_ID  
EDC -> POS: Trans_Type + Host_ID + ECR_Response_Code + EDC_Terminal_ID + SaleAmount + RefundAmount + Sale_Count + Refund_Count + Host_Resp_Code + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且結帳成功後回傳。

### 8.12 Settle (一般結帳: 全國性繳費)
POS -> EDC: Trans_Type ("50") + Host_ID  
EDC -> POS: Trans_Type + Host_ID + ECR_Response_Code + EDC_Terminal_ID + NationalPayAmount + NormalPayAmount + NationalPayCount + NormalPayCount + Host_Resp_Code + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且結帳成功後回傳。

### 8.13 Auto Settle (連動結帳)
POS -> EDC: Trans_Type ("51")  
EDC -> POS: Trans_Type + Host_ID + TCB_ECR_Response_Code + EDC_Terminal_ID + TCB_SETTLE_ESC_Status + TCB_SETTLE_ESC_Response_Code + FISC_SETTLE_ESC_Status + FISC_SETTLE_ESC_Response_Code + NP_SETTLE_ESC_Status + NP_SETTLE_ESC_Response_Code + AE_SETTLE_ESC_Status + AE_SETTLE_ESC_Response_Code + TCB SaleAmount + TCB RefundAmount + TCB Sale_Count + TCB Refund_Count + TCB_Host_Resp_Code + FISC_ECR_Response_Code + FISC SaleAmount + FISC RefundAmount + FISC Sale_Count + FISC Refund_Count + FISC_Host_Resp_Code + NP_ECR_Response_Code + NP SaleAmount + NP RefundAmount + NP Sale_Count + NP Refund_Count + NP_Host_Resp_Code + INST_ECR_Response_Code + INST SaleAmount + INST RefundAmount + INST Sale_Count + INST Refund_Count + INST_Host_Resp_Code + AE_ECR_Response_Code + AE SaleAmount + AE RefundAmount + AE Sale_Count + AE Refund_Count + AE_Host_Resp_Code + CMAS_ECR_Response_Code + CMAS SaleAmount + CMAS RefundAmount + CMAS Sale_Count + CMAS Refund_Count + CMAS_Host_Resp_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且結帳成功後回傳。

### 8.14 Redeem Sale (紅利銷售) 一段式連線
POS -> EDC: Trans_Type ("80") + Trans_Amount  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + Redeem_ActNum + Redeem_Config + Redeem_Resp + Sign_Point_of_Balance + ECR_Response_Code + EDC_Terminal_ID + Reference_No + Amount_After_Cost + Store_Id + CardType + Encrypted_Card_Number + Redeem_Point_Remain + Redeem_Point_Cost + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.15 Redeem Refund (紅利退貨) 一段式連線
POS -> EDC: Trans_Type ("81") + Trans_Amount + Approval_No + Amount_After_Cost + Org_Trans_Date + Redeem_Point_Cost  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + Redeem_ActNum + Redeem_Config + Redeem_Resp + Sign_Point_of_Balance + ECR_Response_Code + EDC_Terminal_ID + Reference_No + Amount_After_Cost + Store_Id + CardType + Org_Trans_Date + Encrypted_Card_Number + Redeem_Point_Remain + Redeem_Point_Cost + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.16 Fisc Refund (金融卡退貨)
POS -> EDC: Trans_Type ("27") + Trans_Amount + Reference_No  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + ECR_Response_Code + EDC_Terminal_ID + Reference_No + Issuer_ID + CardType + Host_Response_Code + EDC_Merchant_ID + ESC_Status + ESC_Response_Code

Notes:
- ESC_Status/ESC_Response_Code 僅在 ECR 電簽上傳回傳開關開啟且交易成功後回傳。

### 8.17 Trans Return (交易回傳)
POS -> EDC: Trans_Type ("91") + Host_ID + Invoice_No  
EDC -> POS: Trans_Type + Host_ID + Invoice_No + Card_No + Trans_Amount + Trans_Date + Trans_Time + Approval_No + ECR_Response_Code + EDC_Terminal_ID + Reference_No + Card_Type + Encrypted_Card_No + CancelDebtNo + Host_Response_Code + EDC_Merchant_ID

Notes:
- 刷卡機請回傳實際交易別 Trans_Type。  
- 請依據原交易類型回傳授權結果。  
- 若收銀機沒給 Invoice_No 則回傳最後一筆。

### 8.18 Print Statement (列印帳務明細)
POS -> EDC: Trans_Type ("52") + Host_ID  
EDC -> POS: Trans_Type + Host_ID + Trans_Date + Trans_Time + ECR_Response_Code + EDC_Terminal_ID + EDC_Merchant_ID

### 8.19 UserLOGON Return (使用者登入回傳)
POS -> EDC: Trans_Type ("92")  
EDC -> POS: Trans_Type + Trans_Date + Trans_Time + ECR_Response_Code + EDC_Terminal_ID + EDC_Merchant_ID

Notes:
- ECR_Response_Code 0000 表示使用者登入成功，0001 表示使用者登入失敗。

### 8.20 INTELLA OLPAY (英特拉主掃)
POS -> EDC: Trans_Type ("36") + Trans_Amount  
EDC -> POS: Trans_Type + Trans_Amount + Trans_Date + Trans_Time + Order_Num + Order_Status + ECR_Response_Code + INTELLA_Response_Code

### 8.21 INTELLA MICROPAY (英特拉被掃)
POS -> EDC: Trans_Type ("37") + Trans_Amount + Scan_Data  
EDC -> POS: Trans_Type + Trans_Amount + Trans_Date + Trans_Time + Order_Num + ECR_Response_Code + INTELLA_Response_Code

Notes:
- 若收銀機沒給 Scan_Data 則由刷卡機進行掃碼。

### 8.22 INTELLA REFUND (英特拉退款)
POS -> EDC: Trans_Type ("38") + Trans_Amount + Order_Num  
EDC -> POS: Trans_Type + Trans_Amount + Trans_Date + Trans_Time + Order_Num + ECR_Response_Code + INTELLA_Response_Code

Notes:
- 若收銀機沒給 Order_Num 則由刷卡機進行輸入。

### 8.23 INTELLA INQUIRY (英特拉查詢)
POS -> EDC: Trans_Type ("39") + Order_Num  
EDC -> POS: Trans_Type + Trans_Amount + Trans_Date + Trans_Time + Order_Num + Order_Status + ECR_Response_Code + INTELLA_Response_Code

Notes:
- 若收銀機沒給 Order_Num 則由刷卡機進行輸入。

## 9. Communication Error Flows (PDF p.48-50)

- Request LRC/Length error: EDC sends NAK (twice). POS retries request up to 3 times; then communication fails.
- EDC Ack Lose: POS timeout (1s) waiting ACK/NAK after Request, assumes ACK and continues.
- POS Ack Lose: If EDC does not receive ACK/NAK within 1s or total >5s after Response, EDC assumes NAK and may resend. POS must handle duplicates safely.
