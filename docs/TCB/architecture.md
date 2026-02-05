# TCB POS System Architecture (Server_TCB)

## 1. Overview

本架构沿用现有 ECPay 服务器的**分层与解耦原则**，但严格对齐 TCB RS232 规范（Ver 3.6）及 `TCB_FSM.md`。
目标是提供一个可落地的本地中间件：

- 上层：Webapp / Electron / 业务系统（WebSocket）
- 中间：Go Server（协议、FSM、串口驱动）
- 下层：TCB POS 终端（RS232 115200 8N1）

核心原则：
1. **协议正确性优先**（ACK/NAK 双发、ACK Lose 逻辑、重试规则）
2. **业务与链路解耦**（Link 层只处理字节流可靠传输）
3. **幂等与去重**（避免重复入账）
4. **单交易串行执行**（避免并发污染串口）

---

## 2. 环境架构

### 2.1 开发环境（建议）

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Development Environment                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐      TCP :9999      ┌──────────┐   WebSocket  ┌─────────┐ │
│  │ Mock POS │ ◄─────────────────► │ server   │ ◄──────────► │ Webapp  │ │
│  │  (Go)    │   tcp://localhost   │  _tcb    │   :8989/ws   │         │ │
│  └──────────┘                     └──────────┘              └─────────┘ │
│                                                                          │
│  * Mock POS: 使用 mock-pos-tcb（tcp://localhost:9999）                │
│  * Server: 使用 TCPPort / SerialPort 统一接口                           │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.2 生产环境

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Production Environment                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐   RS232 Serial      ┌──────────┐   WebSocket  ┌─────────┐ │
│  │ TCB POS  │ ◄─────────────────► │ server   │ ◄──────────► │ Webapp  │ │
│  │ Terminal │   COMx / ttyUSB0    │  _tcb    │   :8989/ws   │         │ │
│  └──────────┘   115200 bps 8N1    └──────────┘              └─────────┘ │
│                                                                          │
│  * TCB 规范无 ECHO 握手，建议通过配置指定串口                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 3. 组件拆分

### 3.1 Webapp / Electron
- 交易发起与 UI 状态展示
- 与本地服务通过 WebSocket 通信

### 3.2 Server_TCB (Go)

模块划分：
1. **api/** WebSocket 接口与状态广播
2. **driver/** 串口驱动 + FSM + 重试策略
3. **protocol/** 解析/构造 600-byte payload
4. **config/** 配置（端口、超时、扫描策略）
5. **logger/** 日志与审计

### 3.3 POS 终端 (TCB)
- 按 PDF 3.6 处理 RS232 603-byte 帧
- ACK/NAK 双发、ACK Lose 逻辑

---

## 4. 核心协议差异（相较 ECPay）

| 项目 | ECPay | TCB |
| :--- | :--- | :--- |
| ACK/NAK | 单发 | 双发（两次） |
| ACK 超时 | 5s | 1s，超时视为 ACK |
| 重试规则 | 超时/NAK 重试 | 仅 NAK 重试，间隔 1s，最多 3 次 |
| Response 重送 | 无明确 | EDC 5s 内可重送 |
| Payload 解析 | 固定字段 | 多态布局（按 Trans_Type） |
| 交易成功 | RespCode=0000 | ECR_Response_Code=0000，需结合 Host 响应 |
| Hash/签名 | SHA1 | 无 |

---

## 5. TCB FSM 与链路策略

- **Link 层**：严格执行 `TCB_FSM.md`
- **关键要点**：
  1. ACK/NAK 双发
  2. ACK 超时视为成功
  3. 等待 ACK 时允许直接收到 STX
  4. Response 错误回 NAK 并限制重送次数

---

## 6. 交易与业务流

### 6.1 一段式交易（01/02/03/04/30/80/81）
1. Build Request
2. Send → Wait ACK (1s, timeout assume ACK)
3. Wait Response (60s)
4. Parse → 判定成功/失败

### 6.2 二段式交易（60 + 62）
1. GET PAN (`60`)
2. Response 后展示卡信息
3. 完成交易 (`62`)
4. 取消 (`70`)

### 6.3 逾时查询（07）
- 仅在 0011/0013 场景提示用户
- 依 3.1.3/4.x 说明构造查询

---

## 7. JSON Layout 与协议解析

- 解析严格依赖 `docs/TCB/tcb_layout_v36.json` 版本
- 解析步骤：
  1. 读取 Trans_Type
  2. 映射 layout
  3. 解析字段并输出结构化字典

官方澄清已写入 JSON 备注：
- 分期退貨 = 04
- 金融卡退貨回傳不含 CancelDebt/Batch
- 交易回傳/登入回傳以 3.1.12/3.1.13 为准

---

## 8. 安全与幂等

- 禁止记录明文卡号
- 交易结果去重（建议使用 Trans_Type + Invoice_No + Reference_No + STAN + Amount）
- 重复 Response 只回 ACK，不重复入账

---

## 9. 可扩展性与优化方向

1. **Mock POS（TCB）**：用于自动化测试
2. **解析工具生成**：从 JSON 生成字段常量或 Struct
3. **业务插件层**：特店专用逻辑以插件方式扩展

---

## 10. 与 ECPay 架构一致性评估

**合理且可复用**：
- WebSocket API 模式
- SerialManager + FSM 架构
- 串口驱动分层
- 自动状态广播

**需要调整/优化**：
- ACK/NAK 双发与 ACK Lose 的严格实现
- 多态 payload 解析（不能固定 offset）
- 端口探测策略（TCB 无 ECHO，需要配置端口）

---

## 11. server_tcb 实现建议顺序

1. 复制 `server` 为 `server_tcb` 基础骨架
2. 替换 protocol 构造/解析为 TCB 版本
3. 修改 driver 的 ACK/NAK 和超时策略
4. 更新 WebSocket API 字段与命令
5. 编写 TCB Mock POS（可选）
6. 完成测试与日志规范
