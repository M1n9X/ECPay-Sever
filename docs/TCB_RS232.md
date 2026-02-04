# TCB (Taiwan Cooperative Bank) POS RS232 Specification (Master)

> **Source**: 合作金庫 端末機與收銀機連線規格書
> **Version**: 3.7 (Master Polymorphic Map)
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

### 1.1 Protocol Timing

| Parameter | Value | Description |
| :--- | :--- | :--- |
| **ACK Timeout** | **1 second** | Receiver must reply ACK/NAK within 1s |
| **Retry Limit** | **3 times** | (PDF p.7) Max retransmissions on NAK/Timeout |
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

| Code | Type (EN) | Description (ZH) | Layout Mode |
| :--- | :--- | :--- | :--- |
| `01` | **SALE** | 一般交易 (購貨) | **Mode A** |
| `02` | **REFUND** | 退貨交易 | **Mode A** |
| `03` | **INST_SALE** | 分期付款 | **Mode B** |
| `04` | **INST_REFUND** | 分期退貨 | **Mode B** |
| `05` | NATIONAL_PAY | 全國性繳費 | **Mode C** |
| `06` | NORMAL_PAY | 一般繳費 | **Mode C** |
| `07` | INQ_TIMEOUT | 逾時交易查詢 | Standard |
| `11` | BATCH_RETURN | 主機帳務回傳 | Standard |
| `20` | SELF_SALE | 無人自助交易純讀卡 | **Mode A (Variant)** |
| `21` | **NP_READ_CARD** | 無人全國性繳費 (純讀卡) | **Mode D** |
| `22` | ALIPAY_SALE | 支付寶交易 | Standard |
| `23` | ALIPAY_VOID | 支付寶取消 | Standard |
| `24` | ALIPAY_REFUND | 支付寶退貨 | Standard |
| `25` | **SELF_SALE** | 無人自助交易 | **Mode A** |
| `26` | **NP_SELF_SALE**| 無人全國性繳費 | **Mode C** |
| `27` | FISC_REFUND | 金融卡退貨 | **Mode G** |
| `30` | VOID | 取消交易 | **Mode A** |
| `31` | CMAS_SALE | 悠遊卡購貨 | **Mode E** |
| `32` | CMAS_REFUND | 悠遊卡退貨 | **Mode E** |
| `36` | INTEL_OLPAY | 英特拉主掃 | **Mode F** |
| `37` | INTEL_MICROPAY| 英特拉被掃 | **Mode F** |
| `38` | **INTEL_REFUND**| 英特拉退款 | **Mode F** |
| `39` | **INTEL_INQ** | 英特拉查詢 | **Mode F** |
| `50` | **SETTLE** | 結帳交易 | Standard |
| `51` | AUTO_SETTLE | 自動結帳 | **Mode H** |
| `52` | **PRINT_STMT** | 列印帳務明細 | Standard |
| `60` | **GET_PAN** | 讀取卡號 | **Mode A** |
| `62` | SALE_2STAGE | 二段式交易 | **Mode A** |
| `70` | **TERMINATE** | 終止交易 | **Mode A** |
| `80` | REDEEM_SALE | 紅利交易 | **Mode I (Redeem)** |
| `81` | REDEEM_REF | 紅利退貨 | **Mode I (Redeem)** |
| `91` | **TRANS_RET** | 交易回傳 | Standard |
| `92` | **USER_LOGON** | 使用者登入回傳| **Special (Type 92)** |

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
| `07` | DCC | Dynamic Currency Conversion |
| `08` | National Pay | 全國性繳費 (VGHTPE Only) |
| `99` | Other | 其他 |

## 5. Application Payload Definition (Master Polymorphic Map)

**Offset Rule**: All offsets are **0-based** (Start of DATA = 0).

> [!IMPORTANT]
> **Polymorphic Layouts**: TCB protocol uses `Trans_Type` (Offset 0-1) to switch between **8 distinct memory layouts**.
> **Parser Logic**: Read `Trans_Type` first, then branch to the specific layout handler. Do **not** use a single struct.

### 5.0 Layout Overview

| Mode | Trans_Type | Features |
| :--- | :--- | :--- |
| **A** | `01`, `02`, `25`, `30`, `60`, `62`, `70` | **Standard**. Card_No, Amounts, Terminal_ID. |
| **B** | `03`, `04` | **Installment**. Overrides Middle (Amounts/Period). |
| **C** | `05`, `06`, `26` | **National Pay**. Overrides Header (Fee/Seq) & Tail. |
| **D** | `21` | **Kiosk Read**. Overrides Middle (TAC/IC_Memo). |
| **E** | `31`, `32` | **EasyCard**. Overrides Header (Filler) & Tail (Balance). |
| **F** | `36`, `37`, `38`, `39` | **Wallet**. **Destructive** override from Offset 57 (Order/ScanData). |
| **G** | `27` | **FISC Refund**. Tweaks Middle (Batch_Number). |
| **H** | `51` | **Auto Settle**. Completely different counter structure. |
| **I** | `80`, `81` | **Redeem**. Standard Header + Tail Override (Points). |

---

### 5.1 Mode A: Standard Layout

**Applies to**: `01`, `02`, `25`, `30`, `60`, `62`, `70`
*(and others not specified below)*

#### Header

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 0 | 2 | **Trans_Type** | Transaction Code |
| 2 | 2 | **Host_ID** | Bank ID |
| 4 | 6 | Invoice_No | Trace Number |
| 10 | 19 | Card_No | Card PAN |
| 29 | 4 | Exp_Date | `MMYY` |
| 33 | 12 | **Trans_Amount** | Amount |
| 45 | 6 | Trans_Date | `YYMMDD` |
| 51 | 6 | Trans_Time | `HHmmss` |
| 57 | 9 | **Approval_No** | Auth Code |
| 66 | 12 | **Auth_Amount** | Pre-Auth Amount |
| 78 | 4 | Resp_Code | `0000`=Success |
| 82 | 8 | **Terminal_ID** | TID |

#### Middle

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 90 | 12 | **Reference_No** | RRN (Required for Refund) |
| 102 | 12 | **Exp_Amount** | Other Amount |
| 114 | 18 | Store_Id | Counter ID |
| 134 | 3 | Issuer_ID | Card Issuer |
| 137 | 1 | Card_Type | `1`=VISA, `2`=MC... |
| 138 | 4 | Filler | Space |
| 142 | 1 | Only_Credit | `1`:Force Credit |

#### Tail

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 144 | 50 | Enc_Card_No | Encrypted PAN |
| 194 | 16 | **Cancel_Debt** | 銷帳編號 |
| 210 | 6 | **STAN** | Terminal Sequence |
| 294 | 4 | **Host_Resp** | Bank Host Response |
| 298 | 15 | **Merchant_ID** | Merchant ID |
| 318 | 30 | Add_Info | Print Info |

> **Note (Type 20 Kiosk)**: Middle is Standard, but Tail Offset 294 (`Host_Resp`) is replaced by 1-byte `Wave_Flag`.

---

### 5.2 Mode B: Installment

**Applies to**: `03` (Sale), `04` (Refund)
**Base**: Mode A
**Overrides**: Middle Section only (Offsets 66 & 102-143)

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 66 | 12 | **Down_Payment**| 首期金額 (Replaces Auth_Amount) |
| 102 | 12 | **Each_Payment**| 每期金額 (Replaces Exp_Amount) |
| 134 | 2 | **Period** | 期數 (e.g. `03`) |
| 136 | 1 | Filler | |
| 137 | 1 | Card_Type | Same as Std |
| 138 | 6 | **Interest_Amt**| 手續費 (Overlaps Filler/Only_Credit) |

---

### 5.3 Mode C: National Pay

**Applies to**: `05`, `06`, `26`
**Base**: Mode A
**Overrides**: Header (57-81) & Tail (210+)

#### Header Override

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 57 | 7 | **Issuer_Seq_No** | 發卡方序號 |
| 64 | 4 | **Fee_Amt** | 手續費 |
| 68 | 6 | **Process_Code** | 處理碼 |
| 74 | 4 | **Fisc_Code** | 繳費類別碼 |
| 78 | 4 | Resp_Code | Same as Std |

#### Tail Override

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 210 | 4 | **Host_Resp_Code** | **Moved Here** (Std is at 294) |
| 214 | 4 | **NP_Resp_Err** | 全国缴费错误码 |
| 218 | 15 | **Merchant_ID** | Merchant ID |
| 233 | 16 | **NP_Order_No** | 全繳訂單號 |

---

### 5.4 Mode D: Kiosk Read Card

**Applies to**: `21`
**Base**: Mode A
**Overrides**: Middle (90-137)

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 90 | 30 | **IC_Memo** | 晶片卡備註 |
| 120 | 8 | **TAC** | 驗證碼 |
| 128 | 8 | **TCC_Code** | Terminal Check Code |
| 136 | 1 | **Wave_Flag** | `1`=Contactless, `0`=Contact |

---

### 5.5 Mode E: EasyCard

**Applies to**: `31`, `32`
**Base**: Mode A
**Overrides**: Middle (Filler) & Tail (336+)

#### Middle Override

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 102 | 234 | **Filler** | **Huge Filler** (Until Offset 335) |

#### Tail Override (Offset 336+)

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 336 | 1 | **Ticket_Type** | `1`:TCB `2`:iCash `3`:iPASS |
| 337 | 19 | **Card_ID** | Ticket Card ID |
| 356 | 10 | Ticket_Ref_No | Reference |
| 366 | 10 | Ticket_Batch | Batch |
| 396 | 10 | **Balance** | Current Balance |

---

### 5.6 Mode F: Wallet / QR Scan

**Applies to**: `36`, `37`, `38`, `39`
**Type**: **Destructive Override** starting at Offset 57.

> [!CAUTION]
> Do NOT look for `Terminal_ID` or `Reference_No` in standard locations.

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 0 | 2 | Trans_Type | |
| 2 | 2 | Host_ID | |
| 4 | ... | ... | (Std Date/Time/Amount at 33-56) |
| 57 | 20 | **Order_Num** | 訂單號 (Overwrites Approval_No) |
| 77 | 1 | **Order_Status**| `0`:Processing, `1`:Success, `2`:Fail |
| 78 | 4 | ECR_Resp | |
| 82 | 4 | Intella_Resp | |
| 86 | 512 | **Scan_Data** | **QR Payload** (Overwrites Everything) |

---

### 5.7 Mode G: FISC Refund

**Applies to**: `27`
**Base**: Mode A
**Overrides**: Middle (138+)

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 138 | 1 | Card_Type | (Reserved/Filler) |
| 139 | 6 | **Batch_Number** | 原交易批次號 (Fills to 144) |

---

### 5.8 Mode H: Auto Settle

**Applies to**: `51`
**Type**: **Complete Rewrite**

> **Do not use Standard Header**.

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 0 | 2 | Trans_Type | `51` |
| 2 | 2 | Host_ID | |
| 4 | N*Len | **Counters** | Concatenated Counters for 6 Hosts |

**Counter Block Structure** (Repeated for TCB, FISC, NP, AE, INST, CMAS):

* `Resp_Code` (4) + `Status` (1)
* `SaleAmt` (12) + `RefAmt` (12)
* `SaleCnt` (3) + `RefCnt` (3)

---

### 5.9 Mode I: Redeem (Preserved)

**Applies to**: `80`, `81`
**Base**: Mode A
**Overrides**: Tail (194+)

| Offset | Len | Field Name | Description |
| :--- | :-- | :--- | :--- |
| 194 | 10 | **Redeem_Rem** | Remaining Points |
| 204 | 10 | **Redeem_Cost**| Points Deducted |
| 318 | 30 | Add_Info | Print Info |
| *Note* | | | No `Cancel_Debt`, `STAN`, `Host_Resp` |

---

### 5.10 Special: User Logon (Type 92)

**Truncated Layout** (Ends at Offset 41).

| Offset | Len | Field Name |
| :--- | :-- | :--- |
| 0 | 2 | Trans_Type (`92`) |
| 2 | 6 | Date |
| 8 | 6 | Time |
| 14 | 4 | ECR_Resp |
| 18 | 8 | TID |
| 26 | 15 | MID |

---

## 6. Response Code Reference (Offset 78)

| Code | Description (ZH) | Action |
| :--- | :--- | :--- |
| `0000` | 授權 / 成功 | Approve |
| `0001` | 拒絕 | Decline |
| `0004` | 操作錯誤 | Fix Logic |
| `0012` | **請先結帳** | **Must Execute Settle (50)** |

## 7. Implementation Notes

1. **Switch-Case First**: Always switch on `Trans_Type` (Offset 0-1) before parsing deeper fields.
2. **Padding**: Numeric=Right-0-Pad, String=Left-Space-Pad.
3. **LRC**: `XOR(Data[0]...Data[599]) XOR ETX`.
4. **Dates**: TCB uses `YYMMDD` + `HHmmss` (Separated).
5. **Mode F**: Wallet transactions (`36-39`) destroy almost all standard fields.
6. **Mode H**: Auto Settle (`51`) describes machine state, not a transaction.
