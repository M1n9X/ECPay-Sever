### 1. 状态机设计 (Aligned with PDF v3.6)

本设计严格对齐「合庫 EDC POS 連線規格長度600 Ver 3.6」第 47-50 页，覆盖 ACK/NAK 双发、ACK Lose 处理与重传规则。

#### 1.1 链路层状态机 (Link Layer FSM, POS 视角)

*负责 STX/ETX/LRC 封包、ACK/NAK 双发、重传与 ACK Lose 规则。*

| 状态 | 事件 | 动作 | 下一状态 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| **IDLE** | 上层请求发送 | 组包(加LRC) -> 写入串口 | **WAIT_ACK** | 发送 Request |
| | 收到 `STX` | 重置接收Buffer | **READING** | 收到 EDC Response |
| **WAIT_ACK** | 收到 `ACK` | (无) | **IDLE** | 发送成功 |
| | 收到 `NAK` | 重发次数 < 3 ? 重发 : 报错 | **WAIT_ACK** / **IDLE** | 仅在 NAK 时重发 |
| | 超时 1s | 视为 `ACK` | **IDLE** | **EDC Ack Lose** (PDF p.49) |
| | 收到 `STX` | 重置接收Buffer | **READING** | EDC Ack Lose 后可能直接送 Response |
| **READING** | 收到数据字节 | 存入 Buffer | **READING** | 固定长度 600+ETX+LRC |
| | 收满 600+ETX+LRC | 计算 LRC | **VERIFY** | 收到完整包 |
| | 接收长度不足/断流 | 发送 `NAK`x2 | **IDLE** | 视为长度错误 |
| **VERIFY** | LRC/长度正确 | 发送 `ACK`x2 -> 通知应用层 | **IDLE** | 接收成功 |
| | LRC/长度错误 | 发送 `NAK`x2 | **IDLE** | EDC 将重送 (PDF p.49) |

**重复 Response 处理**: 若 POS 未在 1s 内回 ACK/NAK，EDC 可能重送 Response，最久 5s (PDF p.50)。POS 必须容忍重复 Response 并再次回 ACKx2，应用层需做幂等处理。

**重试与延迟规则**:
- Request 端：收到 NAK 后**延迟 1 秒**再重送；最多重送 3 次（共 4 次发送）后判定通信失败。  
- Response 端：若收到 LRC/长度错误，POS 回 NAKx2，EDC 会重送；建议同样以 **3 次** 为上限，超过则报错并回到 IDLE。

**噪声与残留字节处理**:
- 在 IDLE / WAIT_ACK / READING 时收到非 STX 且非 ACK/NAK 的噪声字节应直接丢弃，不进入状态迁移。
- ACK/NAK 为双发，应用应容忍重复 ACK/NAK，不影响状态机稳定性。

#### 1.2 应用层状态机 (App Layer FSM)

| 状态 | 事件 | 动作 | 下一状态 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| **APP_IDLE** | 用户发起交易 | 构造 Request -> Link.Send | **APP_WAIT_RESP** | |
| **APP_WAIT_RESP** | Link 收到 Response | 解析 -> 返回结果 | **APP_IDLE** | 正常流程 |
| | 总超时 60s | 交易逾时错误 | **APP_IDLE** | PDF: POS Timeout > 60s |

**应用层建议实现（上位机行为）**:
- **去重键**: 建议使用 `Trans_Type + Invoice_No + Reference_No + STAN + Trans_Amount + Trans_Date + Trans_Time` 作为幂等键；若字段缺失，则降级组合可用字段（例如仅使用 `Invoice_No + Trans_Amount + Trans_Time`）。  
- **结果判定**: 以 `ECR_Response_Code` 为主，若为 0000 视为成功；若为 0011/0013 等需提示用户“逾时查询/请至柜台”。  
- **展示信息**: 将 `ECR_Response_Code` 映射为人可读错误提示；成功时可展示 `Approval_No`、`Reference_No`、`Terminal_ID`。  
- **重复 Response**: 若去重键已处理过，仍需回 ACKx2，但不得重复入账或重复打印。

#### 1.3 上位机实现指引（可直接落地）

**推荐交易流程（同步式）**:
1. 校验输入参数（交易别、金额、必要字段）。
2. 构造 600-byte DATA，填充必填字段与空白。
3. 调用 Link.Send（发送 Request）。
4. 进入 `APP_WAIT_RESP`，等待 Link.Read 收到 Response。
5. 解析 Response -> 判定成功/失败 -> 显示结果。
6. 记录幂等键与结果，避免重复入账。

**推荐交易流程（异步式）**:
1. UI 发起交易，进入 `PENDING` 状态并显示“处理中”。
2. 发送 Request 后进入等待。
3. 若收到 Response，转 `SUCCESS` 或 `FAILED`，更新 UI。
4. 若超过 60s 未收到，转 `TIMEOUT`，提示“交易逾时，请查询结果”。

**错误提示映射（示例）**:
1. `0000` -> 交易成功
2. `0001` -> 交易失败（拒绝）
3. `0003` -> 操作逾时
4. `0005` -> 通讯失败
5. `0011` -> 連線逾時，请确认网络或改为查询
6. `0013` -> 全國性繳費交易逾時，请进行逾时交易查询

**业务关键字段建议**:
1. 成功交易展示：`Approval_No`、`Reference_No`、`EDC_Terminal_ID`、`Trans_Amount`、`Trans_Date`、`Trans_Time`。
2. 失败交易展示：`ECR_Response_Code` 与人可读说明。
3. 查询/补救交易：保留 `Invoice_No`、`Reference_No` 作为后续查询依据。

**幂等与重复包处理**:
1. 若收到重复 Response（去重键已存在），只回 ACKx2，不重复入账。
2. 若在 `APP_WAIT_RESP` 中先收到 ACK/NAK 噪声，忽略并继续等待 Response。

**超时处理建议**:
1. 交易 60s 超时后显示“交易逾时”，并提供“查询结果”入口。
2. 若收到 `0011` 或 `0013`，提示用户保留交易凭证并进行查询。

#### 1.4 上位机状态机（UI/业务层）

**UI 状态枚举（建议）**:
1. `IDLE` 空闲
2. `INPUT` 输入金额/选择交易
3. `SENDING` 正在发送
4. `WAITING` 等待刷卡/扫码或响应
5. `SUCCESS` 交易成功
6. `FAILED` 交易失败
7. `TIMEOUT` 交易逾时
8. `CANCELLED` 用户取消
9. `ERROR` 通讯或系统错误

**UI 状态迁移（建议）**:
1. `IDLE -> INPUT` 用户开始操作
2. `INPUT -> SENDING` 点击确认
3. `SENDING -> WAITING` Request 已送出
4. `WAITING -> SUCCESS` 收到成功 Response
5. `WAITING -> FAILED` 收到失败 Response
6. `WAITING -> TIMEOUT` 60s 超时
7. `WAITING -> CANCELLED` 发送终止交易或用户取消
8. `* -> ERROR` 串口或系统错误
9. `SUCCESS/FAILED/TIMEOUT/CANCELLED/ERROR -> IDLE` 用户确认结束

#### 1.5 交易类型流程要点

**一段式交易（01/02/03/04/30/80/81 等）**:
1. 构造 Request 直接发送。
2. 等待 Response 并解析。

**二段式交易（60 + 62）**:
1. 发送 `60` 并带 `Start Get PAN`。
2. 收到 Response 后展示读卡结果或提示用户继续。
3. 发送 `62` 完成交易。
4. 若用户取消，发送 `70` 终止。

**逾时查询（07）**:
1. 当出现 `0011` 或 `0013` 时，引导用户进行逾时查询。
2. 查询使用 `07`，需携带 PDF 指定的查询字段（详见 `TCB_RS232.md` 的 3.1.3/3.2 及 4.x 流程说明）。

#### 1.6 关键字段与业务语义

**必须持久化的字段**:
1. `Invoice_No` 签单调阅编号
2. `Reference_No` 银行交易序号
3. `Approval_No` 授权码
4. `Trans_Amount` 交易金额
5. `Trans_Date` / `Trans_Time`
6. `EDC_Terminal_ID` 端末机代号
7. `ECR_Response_Code` 通讯回应码
8. `Host_Response_Code` 主机回应码（若有）

**成功判定建议**:
1. `ECR_Response_Code == 0000` 视为通讯成功。
2. 若有 `Host_Response_Code`，需判断是否为主机成功码。
3. 英特拉交易需同时判断 `ECR_Response_Code` 与 `INTELLA_Response_Code`，且查詢/主掃需判断 `Order_Status`。

#### 1.7 幂等与重复包策略

**幂等键生成**:
1. 首选 `Trans_Type + Invoice_No + Reference_No + STAN + Trans_Amount + Trans_Date + Trans_Time`
2. 若字段缺失，降级组合 `Invoice_No + Trans_Amount + Trans_Time`
3. 保留至少 24 小时或依业务需要持久化

**重复包处理**:
1. 重复 Response 只回 ACKx2，不重复入账、不重复打印。
2. 若重复包携带不同 `ECR_Response_Code`，以首次成功为准并记录差异日志。

#### 1.8 错误处理与提示文案建议

**通讯类**:
1. `0005` 通讯失败 -> “与刷卡机通讯失败，请检查连线”
2. `0011` 连线逾时 -> “交易逾时，请查询结果”

**业务类**:
1. `0001` 拒绝 -> “交易失败，请更换付款方式”
2. `0012` 未结账 -> “请先进行结账”
3. `0013` 全國性繳費逾时 -> “请进行逾时交易查询”

#### 1.9 日志与稽核建议

**建议记录**:
1. 发送时间、接收时间、耗时
2. Request/Response 的关键信息（不保存卡号明文）
3. `ECR_Response_Code`、`Host_Response_Code`、`Approval_No`
4. 失败原因与重试次数

#### 1.10 超时与重试建议

**超时**:
1. Link 层 ACK 超时 1s，应用层总超时 60s。
2. Response ACK Lose 总时长 5s 内需容忍重复 Response。

**重试**:
1. Request 仅在收到 NAK 时重试，间隔 1s，最多 3 次。
2. Response 错误重试建议同样 3 次上限。

#### 1.11 伪代码（端到端流程）

```text
function pay(amount, transType):
  build request payload (600 bytes)
  send request (Link.Send)
  start 60s timer
  while not timeout:
    if response received:
      if dedupKey already processed:
        ack twice and return previous result
      parse response
      if ECR_Response_Code == 0000:
        show success
      else:
        show failure
      persist result
      return
  show timeout
  offer inquiry (07)
```

#### 1.12 端口与线程模型建议

1. 只允许一个交易在串口上进行，避免并发请求。
2. 读写串口建议由单线程/协程负责，业务通过队列调用。
3. 严格区分 Link 层与业务层，Link 层只做字节级传输与 ACK/NAK。

#### 1.13 安全与合规建议

1. 不记录明文卡号，记录加密卡号或尾号即可。
2. 日志中避免输出完整 600-byte payload。
3. 若需调试，使用脱敏或只保留关键字段。

#### 1.14 各交易别必填字段清单（摘要）

以下仅列出**上位机发送 Request 时必须带入**的字段摘要，完整字段与位置请以 `docs/TCB_RS232.md` 为准。

1. **一般交易** `01/02/30`  
必填: `Trans_Type`、`Trans_Amount`、`Host_ID`(依银行)、`Invoice_No`(若要求)。  
可选: `Only_Credit/CUP`、`Start Get PAN`(二段式流程)。  

2. **分期交易** `03/04`  
必填: `Trans_Type`、`Trans_Amount`、`Period`、`Host_ID`、`Invoice_No`。  

3. **全國性/一般繳費** `05/06`  
必填: `Trans_Type`、`Trans_Amount`、`Host_ID`、`Invoice_No`。  

4. **GET PAN** `60`  
必填: `Trans_Type`、`Trans_Amount`、`Start Get PAN`。  
分期需追加 `Period`。  

5. **二段式完成** `62`  
必填: `Trans_Type`、`Trans_Amount`。  

6. **終止交易** `70`  
必填: `Trans_Type`。  

7. **金融卡退貨** `27`  
必填: `Trans_Type`、`Trans_Amount`、`Reference_No`、`Host_ID`。  

8. **結帳** `50`  
必填: `Trans_Type`、`Host_ID`。  

9. **連動結帳** `51`  
必填: `Trans_Type`。  

10. **交易回傳** `91`  
必填: `Trans_Type`、`Host_ID`、`Invoice_No`。  

11. **使用者登入回傳** `92`  
必填: `Trans_Type`。  

12. **英特拉交易** `36/37/38/39`  
必填: `Trans_Type`、`Trans_Amount`（查詢除外）、`Order_Num`（退款/查詢）、`Scan_Data`（被掃）。  

#### 1.15 Request 构造示例（片段）

**示例 1：一般銷售 (01)**  
1. 创建 600-byte 空白 payload。  
2. 写入 `Trans_Type=01`。  
3. 写入 `Trans_Amount` (12 位，右靠左补 0)。  
4. 写入 `Host_ID`、`Invoice_No`（若要求）。  

**示例 2：二段式 (60 + 62)**  
1. 发送 `60`，带 `Trans_Amount`、`Start Get PAN` = `01` 或 `02`。  
2. 收到响应后，发送 `62` 完成交易。  
3. 若用户取消，发送 `70`。  

#### 1.16 上位机模块结构建议

**模块划分**:
1. `LinkLayer`：串口读写、ACK/NAK、LRC 校验、重试。
2. `Protocol`：字段映射、payload 构造与解析（依 `tcb_layout_v36.json`）。
3. `Service`：业务流程编排（支付、退货、结账、查询）。
4. `UI/Presenter`：状态管理与用户提示。
5. `Storage`：幂等键、交易结果、日志落盘。

#### 1.17 模块接口草案（示例）

**LinkLayer**:
1. `Send(payload []byte) error`
2. `Read() ([]byte, error)`

**Protocol**:
1. `BuildRequest(transType, params) []byte`
2. `ParseResponse(payload []byte) (fields, error)`

**Service**:
1. `Pay(amount, hostId) Result`
2. `Refund(amount, refNo) Result`
3. `GetPan(startType) Result`
4. `Settle(hostId) Result`
5. `InquiryTimeout(params) Result`

#### 1.18 UI 交互与信息展示建议

1. 交易进行中显示动画或进度提示，防止用户重复触发。
2. 成功结果展示 `Approval_No`、`Reference_No`、`Amount`、`Date/Time`。
3. 失败结果显示清晰错误原因与下一步动作（如查询或重试）。
4. 逾时提示必须包含“请查询交易结果”。

#### 1.19 统一的 Result 结构建议

1. `status`: `SUCCESS` / `FAILED` / `TIMEOUT` / `CANCELLED` / `ERROR`
2. `message`: 人可读提示
3. `transType`: 交易别
4. `amount`: 交易金额
5. `approvalNo` / `referenceNo` / `terminalId`
6. `ecrCode` / `hostCode` / `intellaCode`
7. `raw`: 原始字段（用于审计/排障）

#### 1.20 各交易别 Request 详细填充表（示例）

**说明**: 以下为常见交易的字段填充示例，仅列出 Request 端必填/常用字段。字段位置与长度请以 `docs/TCB_RS232.md` 为准。

**A. 一般銷售 (01)**  
必填字段:
1. `Trans_Type = "01"`
2. `Host_ID` (例如 `02`)
3. `Trans_Amount` (12 位，右靠左补 0)
4. `Invoice_No` (若设备/银行要求)
常用字段:
1. `Only_Credit/CUP`（强制银联或信用卡）

**B. 一般退貨 (02)**  
必填字段:
1. `Trans_Type = "02"`
2. `Host_ID`
3. `Trans_Amount`
4. `Reference_No`（若要求）

**C. 分期購貨 (03)**  
必填字段:
1. `Trans_Type = "03"`
2. `Host_ID` (分期主机 `04`)
3. `Trans_Amount`
4. `Period` (期数)

**D. 分期退貨 (04)**  
必填字段:
1. `Trans_Type = "04"`
2. `Host_ID`
3. `Trans_Amount`
4. `Period`

**E. 全國性/一般繳費 (05/06)**  
必填字段:
1. `Trans_Type = "05"` 或 `"06"`
2. `Host_ID`
3. `Trans_Amount`

**F. GET PAN (60)**  
必填字段:
1. `Trans_Type = "60"`
2. `Trans_Amount`
3. `Start Get PAN` = `"01"` 或 `"02"`（或 `"03"`/`"04"`）
4. 分期模式需带 `Period`

**G. 二段式完成 (62)**  
必填字段:
1. `Trans_Type = "62"`
2. `Trans_Amount`

**H. 終止交易 (70)**  
必填字段:
1. `Trans_Type = "70"`

**I. 金融卡退貨 (27)**  
必填字段:
1. `Trans_Type = "27"`
2. `Host_ID = "03"`
3. `Trans_Amount`
4. `Reference_No`

**J. 結帳 (50)**  
必填字段:
1. `Trans_Type = "50"`
2. `Host_ID`

**K. 連動結帳 (51)**  
必填字段:
1. `Trans_Type = "51"`

**L. 交易回傳 (91)**  
必填字段:
1. `Trans_Type = "91"`
2. `Host_ID`
3. `Invoice_No`

**M. 使用者登入回傳 (92)**  
必填字段:
1. `Trans_Type = "92"`

**N. 英特拉交易 (36/37/38/39)**  
必填字段:
1. `Trans_Type = "36"|"37"|"38"|"39"`
2. `Trans_Amount`（查詢除外）
3. `Order_Num`（退款/查詢）
4. `Scan_Data`（被掃）

#### 1.21 常用 Request 示例 Payload（伪码）

```text
// 一般交易 (01)
payload = 600 spaces
set(Trans_Type, "01")
set(Host_ID, "02")
set(Trans_Amount, "000000010000")
set(Invoice_No, "ABC123")
Link.Send(payload)

// GET PAN (60) -> SALE (62)
payload = 600 spaces
set(Trans_Type, "60")
set(Trans_Amount, "000000005000")
set(StartGetPAN, "01")
Link.Send(payload)
wait response...

payload = 600 spaces
set(Trans_Type, "62")
set(Trans_Amount, "000000005000")
Link.Send(payload)
```

#### 1.22 上位机事件驱动架构（建议）

**事件类型**:
1. `UiStartTransaction`
2. `UiCancelTransaction`
3. `LinkAckTimeout`
4. `LinkResponseReceived`
5. `LinkError`
6. `AppTimeout`

**事件流示例**:
1. `UiStartTransaction` -> `BuildRequest` -> `Link.Send` -> `WAITING`
2. `LinkResponseReceived` -> `Parse` -> `UpdateUI(SUCCESS/FAILED)`
3. `AppTimeout` -> `UpdateUI(TIMEOUT)` -> `OfferInquiry`

#### 1.23 串口线程模型（建议）

1. 单线程读写串口，避免并发请求。
2. 读写使用阻塞 + 超时机制，避免 CPU 空转。
3. 上位机通过队列投递请求，串口线程按序处理。

#### 1.24 UI 结果页最少显示内容（建议）

1. `交易结果` (成功/失败/逾时)
2. `交易金额`
3. `交易时间`
4. `授权码` (若成功)
5. `参考号` (Reference_No)
6. `端末机编号` (TID)

---

### 2. Go 参考实现 (Aligned with PDF v3.6)

```go
package tcbpos

import (
 "bytes"
 "errors"
 "fmt"
 "io"
 "time"
)

// ==========================================
// 1. Constants & Configuration
// ==========================================

const (
 STX byte = 0x02
 ETX byte = 0x03
 ACK byte = 0x06
 NAK byte = 0x15

 PacketLen    = 603 // STX(1) + Data(600) + ETX(1) + LRC(1)
 PayloadLen   = 600
 AckTimeout   = 1 * time.Second
 AckRepeat    = 2            // ACK/NAK must be sent twice
 MaxRetries   = 3            // retries on NAK only (total 4 sends incl. first)
 TransTimeout = 60 * time.Second
)

// ==========================================
// 2. Link Layer (Transport)
// ==========================================

type TcbLink struct {
 port io.ReadWriter // should support Read/Write with timeout
}

func NewTcbLink(port io.ReadWriter) *TcbLink {
 return &TcbLink{port: port}
}

// CalculateLRC: XOR(Data) ^ ETX (STX not included)
func (link *TcbLink) calculateLRC(payload []byte) byte {
 var lrc byte = 0
 for _, b := range payload {
  lrc ^= b
 }
 lrc ^= ETX
 return lrc
}

func (link *TcbLink) writeAck(b byte) {
 // PDF p.47: ACK/NAK must be sent twice
 link.port.Write([]byte{b, b})
}

func (link *TcbLink) setReadDeadline(d time.Duration) {
 if r, ok := link.port.(interface{ SetReadDeadline(time.Time) error }); ok {
  _ = r.SetReadDeadline(time.Now().Add(d))
 }
}

// SendPacket implements PDF p.48-49
func (link *TcbLink) SendPacket(payload []byte) error {
 if len(payload) != PayloadLen {
  return fmt.Errorf("invalid payload length: %d, expected %d", len(payload), PayloadLen)
 }

 frame := make([]byte, 0, PacketLen)
 frame = append(frame, STX)
 frame = append(frame, payload...)
 frame = append(frame, ETX)
 frame = append(frame, link.calculateLRC(payload))

 for attempt := 0; attempt <= MaxRetries; attempt++ {
  if _, err := link.port.Write(frame); err != nil {
   return err
  }

  // Wait for ACK/NAK within 1s
  if err := link.waitAckOrNak(); err == nil {
   // ACK received or timeout => assume ACK (EDC Ack Lose, PDF p.49)
   return nil
  }
  // err means NAK received -> retry after 1s (PDF p.48)
  time.Sleep(1 * time.Second)
 }

 return errors.New("send packet failed: max retries exceeded on NAK")
}

func (link *TcbLink) waitAckOrNak() error {
 resp := make([]byte, 1)
 deadline := time.Now().Add(AckTimeout)

 for time.Now().Before(deadline) {
  link.setReadDeadline(time.Until(deadline))
  n, err := link.port.Read(resp)
  if err != nil || n == 0 {
   // Timeout -> assume ACK per PDF p.49
   return nil
  }
  if resp[0] == ACK || resp[0] == NAK {
   first := resp[0]
   // ACK/NAK are sent twice; try to drain the duplicate quickly.
   link.setReadDeadline(10 * time.Millisecond)
   _, _ = link.port.Read(resp)
   if first == ACK {
    return nil
   }
   return errors.New("nak")
  }
  // Ignore garbage and keep waiting within 1s
 }

 // Timeout without valid ACK/NAK
 return nil
}

func (link *TcbLink) ReadPacket() ([]byte, error) {
 buffer := make([]byte, 1)

 // 1. Wait for STX
 for {
  if _, err := link.port.Read(buffer); err != nil {
   return nil, err
  }
  if buffer[0] == STX {
   break
  }
 }

 // 2. Read remaining 602 bytes (Data + ETX + LRC)
 raw := make([]byte, PacketLen-1)
 offset := 0
 for offset < len(raw) {
  n, err := link.port.Read(raw[offset:])
  if err != nil {
   link.writeAck(NAK)
   return nil, err
  }
  offset += n
 }

 payload := raw[:PayloadLen]
 etxReceived := raw[PayloadLen]
 lrcReceived := raw[PayloadLen+1]

 // 3. Verify Packet
 if etxReceived != ETX {
  link.writeAck(NAK)
  return nil, errors.New("protocol error: ETX missing")
 }

 calcLrc := link.calculateLRC(payload)
 if calcLrc != lrcReceived {
  link.writeAck(NAK)
  return nil, fmt.Errorf("checksum error: expected %02X got %02X", calcLrc, lrcReceived)
 }

 // 4. Send ACK twice
 link.writeAck(ACK)
 return payload, nil
}

// ==========================================
// 3. Application Layer (Logic & Parsing)
// ==========================================

type TcbClient struct {
 link *TcbLink
}

// Example: Build a Request
func BuildSaleRequest(amount int) []byte {
 payload := bytes.Repeat([]byte(" "), PayloadLen)
 copy(payload[0:], []byte("01"))
 copy(payload[33:], []byte(fmt.Sprintf("%012d", amount)))
 return payload
}
```

---

### 3. 对齐说明

1. **ACK/NAK 双发**: PDF p.47 明确要求 ACK/NAK 发送两次。
2. **EDC Ack Lose**: POS 送出 Request 后 1s 未收到 ACK/NAK，视为 ACK 成功继续等待 Response (PDF p.49)。
3. **POS Ack Lose**: EDC 送出 Response 后 1s 未收到 ACK/NAK，或总时长超过 5s，EDC 视为 NAK 并可能重送 (PDF p.50)。
4. **重试规则**: 仅在收到 NAK 时重送，重送次数为 3 次 (PDF p.48)。
5. **交易超时**: POS Timeout > 60s，Auth Timeout = 60s (PDF p.49-50)。

### 4. PDF Known Issues (原始 PDF 內部矛盾)

以下為 PDF v3.6 內部不一致處，屬交易欄位與版型定義問題，與通訊 FSM 無直接衝突，但實作時需注意：

- Section 4.7 分期退貨一段式流程標示 `Trans_Type ("02")`，但 Section 3.2 明確定義分期退貨為 `04`，且 Start Get PAN 亦標示分期退貨為 `04`。  
- Section 4.16 金融卡退貨回傳欄位清單未包含 `Cancel Debt Number` 與 `Batch_Number`，但 Section 3.1.11 版型明確包含這些欄位。  
- Section 4.17/4.18/4.19 交易回傳、列印明細、使用者登入回傳之欄位清單與 Section 3.1.12/3.1.13 的 600-byte 版型不一致；其中 4.18 未提供完整 600-byte 版型。  
