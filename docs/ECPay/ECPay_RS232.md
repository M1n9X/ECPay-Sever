# ECPay POS RS232 Communication Specification

> **Source**: [ECPay Developers - POS RS232 Documentation](https://developers.ecpay.com.tw/?p=32574)
>
> **Last Verified**: 2026-01-16

## 1. Connection Parameters (Physical Layer)

| Parameter | Value |
| :--- | :--- |
| Interface | RS232 (Serial) |
| Baud Rate | **115200** bps |
| Data Bits | 8 |
| Parity | None (N) |
| Stop Bits | 1 |
| Flow Control | None |

## 2. Protocol Frame Structure (Link Layer)

Communication is asynchronous semi-duplex.
**Total Frame Length: 603 Bytes**

| Byte Index | Length | Name | Value (Hex) | Description |
| :--- | :--- | :--- | :--- | :--- |
| 0 | 1 | **STX** | `0x02` | Start of Text |
| 1-600 | 600 | **DATA**| ASCII | Payload. Padding with Space (`0x20`) if empty. |
| 601 | 1 | **ETX** | `0x03` | End of Text |
| 602 | 1 | **LRC** | Calculated | Longitudinal Redundancy Check |

### 2.1 Control Characters

| Name | Hex | Description |
| :--- | :--- | :--- |
| STX | `0x02` | Start of Text |
| ETX | `0x03` | End of Text |
| ACK | `0x06` | Acknowledgement (Success) |
| NAK | `0x15` | Negative Acknowledgement (Error) |

### 2.2 Handshake Flow

```
PC                                  POS (EDC)
 |------- Request Frame -------->|
 |                                 | (Validates LRC)
 |<-------- ACK (0x06) ----------|
 |                                 | (Processing: User swipes card, etc.)
 |<------- Response Frame -------|
 | (Validates LRC)                 |
 |--------- ACK (0x06) --------->|
```

### 2.3 LRC Calculation

LRC is calculated by XORing all bytes in the **DATA** field AND the **ETX** byte.
**STX is NOT included in the calculation.**

```csharp
byte LRC = 0;
foreach (byte b in dataBytes) {
    LRC ^= b;
}
LRC ^= ETX; // ETX (0x03) is included
```

## 3. Transaction Types (TransType)

| Code | Constant | Description (EN) | Description (ZH) |
| :--- | :--- | :--- | :--- |
| `01` | SALE | Credit Card Sale | 一般交易 |
| `02` | REFUND | Refund Transaction | 退貨交易 |
| `10` | PREAUTH | Pre-Authorization | 預先授權 |
| `11` | AUTH_COMPLE | Pre-Auth Completion | 預先授權完成 |
| `50` | SETTLEMENT | Daily Settlement | 結帳交易 |
| `80` | ECHO | Connection Test | 測試連線狀態 |

## 4. Host ID (HostID / Bank Type)

| Code | Description (EN) | Description (ZH) |
| :--- | :--- | :--- |
| `01` | Credit Card | 信用卡 |
| `02` | Points/Loyalty Redemption | 紅利交易 |
| `03` | Installment | 分期交易 |
| `04` | Voucher/Ticket | 票券交易 |

## 5. Data Payload Field Definition (Application Layer)

The `DATA` field is a fixed **600-byte** ASCII string. Fields are positional.

- **Numeric Fields**: Right-aligned, left-padded with `'0'`.
- **String Fields**: Left-aligned, right-padded with Space (`0x20`).

### 5.1 Complete Field List

| Offset | Length | Field Name (EN) | Field Name (ZH) | Type | Notes |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 0 | 2 | **Trans Type** | 交易別 | String | See Section 3 |
| 2 | 2 | **Host ID** | 銀行別 | String | See Section 4 |
| 4 | 6 | Invoice Number | 調閱編號 | String | Leave empty in Request |
| 10 | 19 | Card Number | 信用卡卡號 | String | Leave empty in Request. Masked in Response. |
| 29 | 2 | **CUP Flag** | 銀聯交易 | String | `00`: General, `01`: UnionPay |
| 31 | 12 | **Trans Amount** | 交易金額 | String | No decimal. Last 2 digits are cents. e.g., `100` = $1.00 |
| 43 | 6 | Trans Date | 交易日期 | String | Response: `YYMMDD` |
| 49 | 6 | Trans Time | 交易時間 | String | Response: `hhmmss` |
| 55 | 6 | Approval Number | 授權碼 | String | Response only. |
| 61 | 4 | **ECR Response Code** | 通訊回應碼 | String | See Section 6 |
| 65 | 8 | Terminal ID | 端末機代號 | String | Response only. |
| 73 | 15 | Merchant ID | 商店代號 | String | Response only. |
| 88 | 20 | **EC Order Number** | 綠界授權單號 | String | **Required for Refund** (Original Order No.) |
| 108 | 18 | Store ID | 櫃號 | String | Optional |
| 126 | 2 | Card Type | 卡片代碼 | String | See Section 7 |
| 128 | 12 | Redeem Amount | 折抵金額 | String | Points transaction only |
| 140 | 10 | Redeem Point | 折抵點數 | String | Points transaction only |
| 150 | 10 | Redeem Balance | 剩餘紅利點數 | String | Points transaction only |
| 160 | 2 | Installment Period | 分期期數 | String | Installment only |
| 162 | 12 | Down Payment Amount | 首期金額 | String | Installment only |
| 174 | 12 | Installment Payment | 每期金額 | String | Installment only |
| 186 | 50 | Encrypted Card Number | 電子發票加密卡號 | String | For e-invoice carrier |
| 236 | 20 | POS Number | POS設備編號 | String | Conditionally required |
| 256 | 236 | Reserve | 保留 | String | Fill with spaces |
| 492 | 14 | **POS Request Time** | 收銀機系統時間 | String | `YYYYMMDDHHmmSS` |
| 506 | 40 | **Request Hash Value** | 發送資料雜湊值 | String | SHA-1 of bytes 0-492 (Uppercase Hex) |
| 546 | 14 | EDC Response Time | 刷卡機系統時間 | String | Response only |
| 560 | 40 | Response Hash Value | 回應資料雜湊值 | String | Response only |

> **Note on Offsets**: The official docs use "位置" (Position) which is **1-based**. The offsets above are **0-based** for programming convenience. To convert: `Offset = Position - 1`.

### 5.2 Hash Calculation (SHA-1)

The `Request Hash Value` (Offset 506) is calculated as follows:

1. Take the first **492 bytes** of the DATA payload (Offsets 0-491).
2. Compute **SHA-1** hash.
3. Convert to **40-character uppercase hexadecimal** string.

```go
// Go Example
payload := data[0:492]
hash := sha1.Sum(payload)
hexString := strings.ToUpper(hex.EncodeToString(hash[:]))
copy(data[506:], hexString)
```

## 6. ECR Response Codes

| Code | Description (EN) | Description (ZH) |
| :--- | :--- | :--- |
| `0000` | Approved | 授權 |
| `0001` | Error / Declined | 拒絕 |
| `0002` | Call Bank | 請聯絡銀行 |
| `0003` | Communication Error | 通訊失敗 |

## 7. Card Type Codes

| Code | Card Brand |
| :--- | :--- |
| `00` | VISA |
| `01` | MASTERCARD |
| `02` | JCB |
| `03` | CUP (UnionPay) |

## 8. Transaction-Specific Notes

### 8.1 Sale (TransType 01)

- `Trans Amount`: Required.
- `EC Order Number`: Leave empty (filled by POS in Response).

### 8.2 Refund (TransType 02)

- `Trans Amount`: Refund amount.
- `EC Order Number`: **Required**. Must contain the original transaction's Order Number.

### 8.3 Settlement (TransType 50)

- `Trans Amount`: Set to `000000000000` (zero).
- Triggers daily batch settlement with bank.

### 8.4 Echo (TransType 80)

- Used for connection testing.
- All amount/order fields can be empty.

## 9. Implementation Timeout Recommendations

| Phase | Recommended Timeout |
| :--- | :--- |
| Wait for ACK | 3-5 seconds |
| Wait for Response (User interaction) | **60-120 seconds** |

## 10. Reference Links

| Page | URL |
| :--- | :--- |
| Introduction | <https://developers.ecpay.com.tw/?p=32574> |
| Communication Specs | <https://developers.ecpay.com.tw/?p=32591> |
| Credit Card Sale | <https://developers.ecpay.com.tw/?p=32597> |
| Points Redemption | <https://developers.ecpay.com.tw/?p=32602> |
| Installment | <https://developers.ecpay.com.tw/?p=32607> |
| Refund | <https://developers.ecpay.com.tw/?p=32612> |
| Pre-Authorization | <https://developers.ecpay.com.tw/?p=32617> |
| Pre-Auth Completion | <https://developers.ecpay.com.tw/?p=32622> |
| Settlement | <https://developers.ecpay.com.tw/?p=32627> |
| Data Format Reference | <https://developers.ecpay.com.tw/?p=32639> |
| General Transaction Fields | <https://developers.ecpay.com.tw/?p=32645> |
| Points/Installment Fields | <https://developers.ecpay.com.tw/?p=32650> |
| Pre-Auth Fields | <https://developers.ecpay.com.tw/?p=32655> |

## 11. Official Documentation Discrepancies & Resolutions

> **Note**: This section documents known errors in the official ECPay developer website (verified as of 2026-01-16) and explains why our implementation deviates from the literal text of the official docs in favor of logical correctness.

### 11.1 Hash Calculation Scope (Request Hash Value)

- **Official Doc Issue**: The text on the website implies the hash calculation includes the `POS Request Time` ("Take the first 492 bytes... inclusive of POS Request Time").
- **Analysis**:
  - `POS Request Time` starts at Offset 492.
  - The range "First 492 bytes" corresponds to Offsets **0 to 491**.
  - If `POS Request Time` were included, the range would be 0 to 505 (492 + 14 bytes), which contradicts the "492 bytes" count.
- **Resolution**: We follow the **Byte Count (492 bytes)** and **Offset Range (0-491)** logic. The Hash covers fields `TransType` through `Reserve`, **EXCLUDING** `POS Request Time`.
- **Status**: Our Server and Mock implementation (`packet.go`, `mock-pos`) correctly implement this exclusion.

### 11.2 Field Length Error (EDC Response Time)

- **Official Doc Issue**: Page `32639` defines `EDC Response Time` (Position 547 / Offset 546) as `String(24)`.
- **Analysis**:
  - Start Offset: 546.
  - Length: 24 bytes -> End Offset: 570.
  - `Response Hash Value` starts at Offset 560.
  - **Result**: A 10-byte overlap would occur, corrupting the data structure.
  - Additionally, the format is `YYYYMMDDHHmmSS` which is exactly 14 characters.
- **Resolution**: Validated as `String(14)`.
- **Status**: Local documentation and code use Length 14.

### 11.3 Duplicate Field Name (Installment Payment)

- **Official Doc Issue**: Page `32639` lists Position 175 as `Down Payment Amount` (首期金額), which duplicates the name of Position 163.
- **Analysis**: Contextual naming conventions and Chinese description (`每期金額`) confirm valid field is "Installment Payment".
- **Resolution**: Renamed to `Installment Payment`.
- **Status**: Corrected in local documentation.
