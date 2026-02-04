# TCB (Taiwan Cooperative Bank) POS RS232 Specification (Master)

> **Source**: 合作金庫 端末機與收銀機連線規格書
> **Version**: 3.6 (2025-04-01)
> **Status**: **Master Copy (Full)**
> **Last Updated**: 2026-02-04

## 1. Connection Parameters (Physical Layer)

| Parameter | Value | Constraint |
| :--- | :--- | :--- |
| **Baud Rate** | **115200** | Default. (Range: 110-115200) |
| **Data Bits** | 8 | |
| **Parity** | None | |
| **Stop Bits** | 1 | |
| **Flow Control**| None | |

### 1.1 Protocol Timing (v3.3+)

| Parameter | Value | Description |
| :--- | :--- | :--- |
| **ACK Timeout** | **1 second** | Receiver must reply ACK/NAK within 1s (previously 2s) |
| **ACK Retry Duration** | **5 seconds** | If no ACK after total 5s, assume failure |
| **Inter-byte Delay** | **5ms** (Optional) | Default `N` = No delay. If enabled, 5ms per byte |

## 2. Packet Structure (Link Layer)

**Fixed Length**: 603 Bytes (600 Bytes Payload)

| Offset | Length | Name | Value | Description |
| :--- | :--- | :--- | :--- | :--- |
| 0 | 1 | **STX** | `0x02` | Start of Text |
| 1 | 600 | **DATA** | ASCII | **Application Payload** (See Section 5) |
| 601 | 1 | **ETX** | `0x03` | End of Text |
| 602 | 1 | **LRC** | Calc | `XOR(DATA[0..599]) ^ ETX` |

## 3. Transaction Types (TransType)

Defined at **DATA Offset 0** (2 bytes).
*Restored full list from PDF Page 27.*

| Code | Type (EN) | Description (ZH) | Note |
| :--- | :--- | :--- | :--- |
| `01` | **SALE** | 一般交易 (購貨) | Standard Credit Card Sale |
| `02` | **REFUND** | 退貨交易 | Credit Card Refund |
| `03` | **INST_SALE** | 分期付款 | Installment Sale |
| `04` | **INST_REFUND** | 分期退貨 | Installment Refund |
| `05` | NATIONAL_PAY | 全國性繳費 | |
| `06` | NORMAL_PAY | 一般繳費 | |
| `07` | INQ_TIMEOUT | 逾時交易查詢 | |
| `11` | BATCH_RETURN | 主機帳務回傳 | |
| `20` | SELF_SALE | 無人自助交易純讀卡 | Kiosk Mode |
| `22` | ALIPAY_SALE | 支付寶交易 | |
| `23` | ALIPAY_VOID | 支付寶取消 | |
| `24` | ALIPAY_REFUND | 支付寶退貨 | |
| `25` | **SELF_SALE** | 無人自助交易 | Same layout as Type 01 |
| `26` | **NP_SELF_SALE**| 無人全國性繳費 | Same layout as Type 05 |
| `27` | FISC_REFUND | 金融卡退貨 | Smart Pay Refund |
| `30` | VOID | 取消交易 | General Void |
| `31` | CMAS_SALE | 悠遊卡購貨 | EasyCard Sale |
| `32` | CMAS_REFUND | 悠遊卡退貨 | EasyCard Refund |
| `36` | INTEL_OLPAY | 英特拉主掃 | Intella |
| `37` | INTEL_MICROPAY| 英特拉被掃 | Intella |
| `38` | **INTEL_REFUND**| 英特拉退款 | Wallet Refund |
| `39` | **INTEL_INQ** | 英特拉查詢 | Wallet Inquiry |
| `50` | **SETTLE** | 結帳交易 | Batch Settlement |
| `51` | AUTO_SETTLE | 自動結帳 | **Special Layout** (See Note) |
| `52` | **PRINT_STMT** | 列印帳務明細 | |
| `60` | **GET_PAN** | 讀取卡號 | Read Card No (No Charge) |
| `62` | SALE_2STAGE | 二段式交易 | Follows Type 60 |
| `80` | REDEEM_SALE | 紅利交易 | Points Redemption Sale |
| `81` | REDEEM_REF | 紅利退貨 | Points Redemption Refund |
| `91` | **TRANS_RET** | 交易回傳 | Upload last transaction |
| `92` | **USER_LOGON** | 使用者登入回傳| **Truncated Layout** (See 5.4) |

## 4. Host ID (Bank Type)

Defined at **DATA Offset 2** (2 bytes).

| Code | Bank | Note |
| :--- | :--- | :--- |
| `00` | Citi/Diners | 花旗/大來 |
| `01` | AMEX | 美國運通 |
| `02` | TCB | 合庫 (Credit / Redeem) |
| `03` | Fisc | 財金 (Smart Pay) |
| `04` | TCB Inst | 合庫 (Installment) |
| `05` | EasyCard | 悠遊卡 |
| `06` | iPASS | 一卡通 |
| `08` | National Pay | 全國性繳費 (VGHTPE Only) |
| `99` | Other | 其他 |

## 5. Application Payload Definition

**Offset Rule**: All offsets are **0-based** (Start of DATA = 0).
**Format**: Numeric (`N`) = Right-aligned 0-padded. String (`S`) = Left-aligned Space-padded.

### 5.1 Common Header (Offsets 0-133)

| Offset | Len | Field Name | Type | Req | Resp | Description |
| :--- | :-- | :--- | :--- | :--- | :--- | :--- |
| 0 | 2 | **Trans_Type** | N | M | M | Transaction Code |
| 2 | 2 | **Host_ID** | N | O | M | Bank ID |
| 4 | 6 | Invoice_No | S | O | M | Trace Number (調閱編號) |
| 10 | 19 | Card_No | S | - | M | Card PAN |
| 29 | 4 | Exp_Date | S | - | - | `MMYY` or Space |
| 33 | 12 | **Trans_Amount** | N | M | M | Amount (No decimal) |
| 45 | 6 | Trans_Date | N | - | M | `YYMMDD` |
| 51 | 6 | Trans_Time | N | - | M | `HHmmss` |
| 57 | 9 | Approval_No | S | - | M | Auth Code |
| **66** | 12 | **(Union A)** | - | - | - | *See 5.2 (AuthAmt / DownPayment)* |
| 78 | 4 | **Resp_Code** | S | - | M | `0000`=Success |
| 82 | 8 | Terminal_ID | S | - | M | TID |
| 90 | 12 | Reference_No | S | C | M | RRN. **Required for Refund**. |
| **102**| 12 | **(Union B)** | - | - | - | *See 5.2 (ExpAmt / EachPayment)* |
| 114 | 18 | Store_Id | S | O | O | Counter ID |
| 132 | 2 | Start_Get_PAN | N | C | - | Only for Type `60` |

### 5.2 Polymorphic Fields (Offsets 66, 102, 134-143)

These fields change definition based on `Trans_Type`.

#### Case A: Standard Sale / Refund (Types 01, 02)

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 66 | 12 | Auth_Amount | Pre-Auth Amount (Default 0/Space) |
| 102 | 12 | Exp_Amount | Other Amount (Default 0/Space) |
| 134 | 3 | Issuer_ID | Card Issuer |
| 137 | 1 | Card_Type | `1`:Visa `2`:Master `3`:JCB `6`:AE `8`:CUP `9`:Other |
| 138 | 4 | Filler | Space |
| 142 | 1 | Only_Credit | `1`:Force Credit, `2`:Force CUP |
| 143 | 1 | *Filler* | Reserved (1-byte gap before Enc_Card_No) |

#### Case B: Installment (Types 03, 04)

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 66 | 12 | **Down_Payment**| 首期金額 |
| 102 | 12 | **Each_Payment**| 每期金額 |
| 134 | 2 | **Period** | 期數 (e.g., `03`) |
| 136 | 1 | Filler | Space |
| 137 | 1 | Card_Type | Same as Case A |
| 138 | 6 | **Interest_Amt**| 手續費 (Overlaps Filler/Only_Credit) |

#### Case C: Redeem / Bonus Points (Types 80, 81)

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 66 | 6 | Act_Num | Activity Number |
| 72 | 1 | Config | Redeem Config |
| 73 | 2 | Resp | Redeem Response |
| 75 | 1 | Sign | Balance Sign |
| 76 | 2 | *Filler* | |
| 102 | 12 | Amt_After_Cost| Amount after redemption |

**Tail Layout Override for Redeem (Offset 194+):**

> [!WARNING]
> The Tail Section (Offsets 194+) is **redefined** for Redeem transactions. Standard fields `Cancel_Debt`, `STAN`, `Host_Resp_Code`, and `Merchant_ID` are **NOT AVAILABLE**.

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 194 | 10 | **Redeem_Rem** | Point Balance (Remaining) |
| 204 | 10 | **Redeem_Cost**| Points Deduction |
| 214 | 104 | *Filler* | Reserved |
| 318 | 30 | Add_Info | Print Info |

---

#### Case D: Wallet / Intella Transactions (Types 36, 37, 38, 39)

> [!CAUTION]
> These transaction types **DO NOT** follow the standard layout after Offset 57. The `Scan_Data` field (for QR codes) **obliterates** standard fields like `Terminal_ID`, `Reference_No`, etc.
>
> When `Trans_Type` is `36/37/38/39`, **DO NOT** try to parse `Card_No` at Offset 10 or `Reference_No` at Offset 90. They do not exist.
>
> **Note**: Implementation should strictly use Types **36-39** as defined in the Code table (PDF Page 27). Type 14 appears in older/internal contexts but is not recommended.

**Layout for Intella/Wallet (Offsets 0-600):**

| Offset | Len | Field Name | Description | Note |
| :--- | :-- | :--- | :--- | :--- |
| 0 | 2 | Trans_Type | Transaction Code | Same as Std |
| 2 | 31 | *Filler* | **Reserved** | **Differs from Std** (Std uses HostID/Invoice/CardNo here) |
| 33 | 12 | Trans_Amount | Amount | Same as Std |
| 45 | 6 | Trans_Date | Date | Same as Std |
| 51 | 6 | Trans_Time | Time | Same as Std |
| 57 | 20 | Order_Num | **Order Number** | **Overwrites** Approval_No(9) + Auth_Amt(12) |
| 77 | 1 | Order_Status | Status | See table below |
| 78 | 4 | Resp_Code | ECR Response | Same as Std |
| 82 | 4 | **Intella_Resp**| Intella Response | **Overwrites** start of Terminal_ID |
| 86 | 512 | **Scan_Data** | **QR/Scan Payload** | **Huge Field**. Overwrites Offsets 86-597. |
| 598 | 2 | *Filler* | | |

**Order_Status Values (Offset 77):**

| Value | Status | Description |
| :--- | :--- | :--- |
| `0` | Processing | 處理中 |
| `1` | Transaction Success | 交易成功 |
| `2` | Transaction Fail | 交易失敗 |
| `3` | Refund Success | 退款成功 |
| `4` | Refund Fail | 退款失敗 |
| `5` | Order Not Found | 訂單資料不存在 |

### 5.3 Tail Section (Offsets 144-600)

| Offset | Len | Field Name | Req | Resp | Description |
| :--- | :-- | :--- | :--- | :--- | :--- |
| 144 | 50 | **Enc_Card_No**| C | C | Base64 Encrypted Card (E-Invoice) |
| 194 | 16 | Cancel_Debt | C | C | 銷帳編號 |
| 210 | 6 | STAN | - | C | Terminal Sequence No |
| 216 | 16 | Order_Num | C | C | Mobile Pay Order No |
| 232 | 16 | **Old_Order_No**| C | C | Original Order No (for Refund) |
| 248 | 14 | Old_Trans_Time| - | C | Orig. Time `YYYYMMDDHHmmss` |
| 262 | 32 | Alipay_ID | - | C | Alipay Trans ID |
| 294 | 4 | **Host_Resp** | - | M | Bank Host Response Code |
| 298 | 15 | Merchant_ID | - | M | Merchant ID |
| 313 | 5 | Filler | - | - | |
| 318 | 30 | Add_Info | O | O | Print Line / Info |
| 348 | 1 | **ESC_Status** | - | C | E-Sig: `0` None, `1` Upload, `2` Print |
| 349 | 4 | ESC_Resp | - | C | E-Sig Response |
| 353 | 247 | Filler | - | - | Space Padding |

> **Note on Redeem (Type 80/81)**:
> For Redeem transactions, offsets **194-318** have specific point definitions. See **Case C Tail Layout Override** in Section 5.2 for complete field definitions.

### 5.4 Special Layouts

#### Type 51 (AUTO_SETTLE) Response Layout

> [!IMPORTANT]
> Type 51 returns a massive block of concatenated counters. **DO NOT** parse using Standard Header offsets after Offset 2.

* Starts at Offset 3 (after HostID).
* Contains concatenated `Response_Code` (4 bytes) and `Status` (1 byte) for **all** host types (TCB, FISC, NP, AE, etc.).
* Followed by `SaleAmount`, `RefundAmount`, `SaleCount`, `RefundCount` for every host sequentially.

---

#### Type 92 (USER_LOGON) Truncated Layout

> [!IMPORTANT]
> Type 92 uses a **truncated layout**. It does NOT contain Amounts, Card Numbers, or Invoice Numbers.

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 0 | 2 | Trans_Type | `92` |
| 2 | 6 | Trans_Date | `YYMMDD` |
| 8 | 6 | Trans_Time | `HHmmss` |
| 14 | 4 | ECR_Resp | ECR Response Code |
| 18 | 8 | EDC_Terminal | Terminal ID |
| 26 | 15 | EDC_Merchant | Merchant ID |

> Parsers should handle this reduced header length to avoid reading garbage data.

## 6. Response Code Reference (Offset 78)

| Code | Description (ZH) | Action |
| :--- | :--- | :--- |
| `0000` | 授權 / 成功 | Approve |
| `0001` | 拒絕 | Decline |
| `0002` | 請聯絡銀行 | Refer to Issuer |
| `0003` | 操作逾時 | Retry |
| `0004` | 操作錯誤 | Fix Logic |
| `0005` | 通訊失敗 | Comm Error |
| `0011` | 連線逾時 | Timeout |
| `0012` | **請先結帳** | **Must Execute Settle (50)** |
| `0013` | 繳費逾時 | Query status at counter |
| `0014` | 查無交易 | Check RRN / Date |

## 7. Implementation Notes

1. **Padding**:
    * Amounts: Right-aligned, pad with `'0'`. (e.g., `000000001200`)
    * Strings: Left-aligned, pad with Space `0x20`.
2. **Date/Time**: TCB uses separated `YYMMDD` (6 bytes) and `HHmmss` (6 bytes), unlike ECPay's 14-byte format.
3. **LRC Logic**: `LRC = XOR(Data[0]...Data[599]) XOR ETX(0x03)`.
4. **Installment Conflict**: When parsing `TransType=03`, do NOT read `Only_Credit` at Offset 142. It is dirty memory because `Interest_Amt` occupies Offset 138-144 (6 bytes).
5. **Wallet/Intella Conflict**: When parsing `TransType=36/37/38/39`, the entire layout after Offset 2 is different. `Scan_Data` (512 bytes at Offset 86) overwrites most standard fields. See **Case D** in Section 5.2.
6. **Redeem Conflict**: When parsing `TransType=80/81`, Tail Section fields (Offset 194+) are redefined. `Cancel_Debt`, `STAN`, `Host_Resp_Code`, and `Merchant_ID` do not exist. See **Case C Tail Layout Override** in Section 5.2.
7. **FISC Refund Rules (Type 27)**:
    * Refund Amount must be **<=** Original Transaction Amount.
    * A single original transaction (`STAN` / `Old_Order_No`) can only be refunded **once**. Subsequent attempts will be rejected by the FISC host.
8. **AUTO_SETTLE (Type 51)**: Do not parse using Standard Header. See Section 5.4.
9. **USER_LOGON (Type 92)**: Uses truncated layout without Amount/CardNo/Invoice fields. See Section 5.4.
