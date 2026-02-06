# Mock POS TCB 行為矩陣（對齊 PDF v3.6）

本矩陣描述 `mock-pos-tcb` 的**預設行為**，以 PDF v3.6 為準。  
若啟用額外開關（如批次/時效限制），將在「備註/例外」欄標示。

## 1. 交易別與流程

| 交易別 | 名稱 | 請求必要欄位 | 回傳必要欄位 | 驗證/約束 | 備註/例外 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| 01 | Sale 銷售 | Trans_Type, Trans_Amount | Invoice_No, Card_No, Trans_Amount, Trans_Date, Trans_Time, Approval_No, ECR_Response_Code, EDC_Terminal_ID, Reference_No, Host_Response_Code, EDC_Merchant_ID | 無 | 依 3.1.1 一般交易版型 |
| 02 | Refund 退貨 | Trans_Type, Trans_Amount | 同 01 | 參照欄位可選 | 依 3.1.1 一般交易版型 |
| 03 | Inst Sale 分期購貨 | Trans_Type, Trans_Amount, Period | 同 01 + DownPayment + EachPayment + Period | 無 | 依 3.1.2 |
| 04 | Inst Refund 分期退貨 | Trans_Type, Trans_Amount, Reference_No | 同 03 | 必須帶 Reference_No | 依 3.1.2，官方澄清 Trans_Type=04 |
| 27 | FISC Refund 金融卡退貨 | Trans_Type, Trans_Amount, Reference_No | 3.1.11 版型 | 金額不得 > 原交易；同序號僅可退一次 | 不回傳 Batch/Cancel（依官方澄清） |
| 60 | GET PAN | Trans_Type, Trans_Amount, Start Get PAN | Trans_Type, Host_ID, Card_No, Trans_Amount, Trans_Date, Trans_Time, ECR_Response_Code, EDC_Terminal_ID | Start Get PAN 必須 01/02/03/04 | 依 3.1.1 (60 交易) |
| 62 | COMPLETE | Trans_Type, Trans_Amount | 同對應的 Sale/Refund 版型 | 必須先有 60；金額 ≤ 60 | 依 3.1.1 / 3.1.2 |
| 70 | TERMINATE | Trans_Type | ECR_Response_Code | 無 | 模擬成功/失敗 |
| 91 | Trans Return | Trans_Type + Invoice_No(可選) | 3.1.12 版型 | 無 Invoice_No 則回傳最近一筆 | 依 3.1.12 備註 |
| 92 | User Login | Trans_Type | 3.1.13 版型 | ECR_Response_Code=0000/0001 | 依 3.1.13 |

## 2. 關鍵回傳欄位（預設）

Mock 預設回傳以下欄位（若版型包含）：

- `Invoice_No`
- `Reference_No`
- `Approval_No`
- `Card_No`（遮掩卡號，6+6+4 格式）
- `CardType`（預設 `2`）
- `Trans_Date` / `Trans_Time`
- `ECR_Response_Code`
- `Host_Response_Code`
- `EDC_Terminal_ID`
- `EDC_Merchant_ID`

補充：
- `Reference_No` 預設以 `Trans_Date + Trans_Time` 組成（12 位），不與 `Invoice_No` 共用。  
- `Card_No` 預設為「6 位 + 6 個 * + 4 位」格式，無連字符，右側補空白至欄位長度。  
- `Approval_No` 若未提供，預設使用 `Trans_Time`。  

## 3. 可選開關（非 PDF 強制）

這些開關為「更嚴格」設備行為，**預設關閉**：

| 開關 | 說明 | 預設 |
| :--- | :--- | :--- |
| `-require-batch-or-cancel` | 退款需批次/銷帳比對 | false |
| `-refund-max-age-hours` | 退款時效限制（超時拒絕） | 0 |
| `-require-ref-and-card` | 參照欄位 + 刷卡同時要求 | false |

## 4. 錯誤行為（模擬）

- `ECR_Response_Code=0001`：一般拒絕（例如參照欄位缺失、找不到原交易）  
- `0011/0013` 未做完整模擬（若需要可擴充）  
