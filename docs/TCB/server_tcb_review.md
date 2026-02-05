# Server_TCB Code Review & Production Readiness

本文件是对 `server_tcb` 的完整 review 结论，重点评估状态机完整性、与 `docs/TCB/architecture.md` 和 `docs/TCB/TCB_FSM.md` 的一致性，以及 Production 使用风险点。

## 1. Review 范围

- 代码目录：`server_tcb/`
- 参考文档：
  - `docs/TCB/architecture.md`
  - `docs/TCB/TCB_FSM.md`
  - `docs/TCB/TCB_RS232.md`
  - `docs/TCB/tcb_layout_v36.json`

## 2. 状态机完整性评估（对齐 TCB_FSM）

当前实现的状态机包含：`IDLE -> SENDING -> WAIT_ACK -> WAIT_RESPONSE -> PARSING -> SUCCESS/ERROR/TIMEOUT`，并支持 `ABORT`。

对齐项：
- ACK/NAK 双发（`driver/manager.go` 的 `writeAck()`）  
- ACK 1s 超时视为成功（`waitAckOrStx`）  
- 等 ACK 时允许直接收到 STX（`waitAckOrStx` 的 `STX` 分支）  
- Response 错误时 NAKx2 并重试（`waitForResponse` + `ResponseMaxRetry`）  
- 60s Response 超时（`ResponseTimeout`）  
- 单交易串行（`api` 层 mutex）  

结果判定：
1. `ECR_Response_Code == 0000` 视为通讯成功。  
2. 若有 `Host_Response_Code` 或 `Host_Resp_Code`，非成功码视为失败。  
3. 若 `ECR_Response_Code` 为 `0011/0013`，视为逾时并建议使用查询。  
4. 对于变体字段（`TCB_ECR_Response_Code` / `FISC_ECR_Response_Code` 等），使用通用匹配。  

结论：**状态机与 FSM 规范一致，可用于 Production 场景。**

## 3. 架构一致性评估（对齐 architecture.md）

| 架构点 | 现状 | 说明 |
| :--- | :--- | :--- |
| ACK/NAK 双发 | 已实现 | 链路层完全对齐 |
| ACK Lose 处理 | 已实现 | 1s timeout 视为 ACK |
| 多态 payload 解析 | 已实现 | `tcb_layout_v36.json` 驱动 |
| 单交易串行 | 已实现 | WebSocket handler mutex |
| 重送/重复包处理 | 已实现 | 5s 视窗内 ACK 重复 Response |
| 幂等/去重（业务层） | 部分实现 | 当前只处理链路重复包，业务侧建议做二次幂等 |
| 日志 | 已实现 | `logger` 模块输出与轮转 |

结论：**架构关键能力已落地，少数“业务幂等”仍需上位机补充。**

## 4. API 与模块设计

- `api/`：WebSocket 指令支持 `SALE/REFUND/SETTLEMENT/GETPAN/COMPLETE/TERMINATE/INQUIRY/TRANSACT`，覆盖 TCB 主要交易。  
- `driver/`：串口驱动与 FSM 完全分离。  
- `protocol/`：解析与构包以 JSON Layout 驱动，避免硬编码 offset。  
- `config/`：生产环境建议指定 `-port` 并禁用 `-autodetect`。  

## 5. 已知约束与建议

1. **主机成功码**：目前认为 `Host_Response_Code` 为 `00/0000` 才成功，如银行另有特殊成功码需扩展。  
2. **业务幂等**：服务器层只保证“同一交易链路内的重复包不会触发重复入账”；业务系统仍需做订单级幂等控制。  
3. **RESTART 指令**：`RESTART` 会直接 `os.Exit(0)`，生产若无守护进程应禁用。  

## 6. 结论

`server_tcb` 当前实现已满足 Production 所需的链路层与协议要求，FSM 完整且与文档一致。  
若后续出现银行特定成功码或更严格的幂等要求，可按上述建议扩展。
