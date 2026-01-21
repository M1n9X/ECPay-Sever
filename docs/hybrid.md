# ECPay POS 混合架构设计方案

## 1. 概述

### 1.1 背景

原有架构将 Webapp 部署在本地 Windows 系统上，导致所有 credentials（API 密钥、商户信息等）暴露在客户端，存在严重安全隐患。

### 1.2 设计目标

- **安全性**：Credentials 永不离开云端
- **可靠性**：支持离线降级，网络恢复后自动同步
- **易部署**：单一安装包，用户无需技术背景
- **可维护**：支持远程更新和日志收集

### 1.3 核心思路

采用 Electron 封装本地应用，Go Server 作为子进程处理硬件通信，Cloud Function 处理敏感业务逻辑。

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CLOUD LAYER                                     │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                     Cloud Function (API Gateway)                       │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │  │
│  │  │    Auth     │  │ Credentials │  │  Business   │  │    Logs     │  │  │
│  │  │   Service   │  │   Vault     │  │   Logic     │  │  Collector  │  │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘  │  │
│  └───────────────────────────────────┬───────────────────────────────────┘  │
└──────────────────────────────────────┼──────────────────────────────────────┘
                                       │ HTTPS (TLS 1.3)
                                       │
┌──────────────────────────────────────┼──────────────────────────────────────┐
│                          CLIENT LAYER (Windows)                              │
│  ┌───────────────────────────────────┼───────────────────────────────────┐  │
│  │                    ELECTRON APPLICATION                                │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │                    MAIN PROCESS (Node.js)                        │  │  │
│  │  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐    │  │  │
│  │  │  │  Cloud    │  │ Process   │  │   IPC     │  │   Auto    │    │  │  │
│  │  │  │  Client   │  │ Manager   │  │  Bridge   │  │  Updater  │    │  │  │
│  │  │  └───────────┘  └─────┬─────┘  └───────────┘  └───────────┘    │  │  │
│  │  └───────────────────────┼─────────────────────────────────────────┘  │  │
│  │                          │ Child Process (spawn)                       │  │
│  │                          ▼                                             │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │                    GO SERVER (Compiled Binary)                   │  │  │
│  │  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐    │  │  │
│  │  │  │ WebSocket │  │  Serial   │  │ Protocol  │  │   State   │    │  │  │
│  │  │  │   API     │  │  Driver   │  │  Handler  │  │  Machine  │    │  │  │
│  │  │  └───────────┘  └─────┬─────┘  └───────────┘  └───────────┘    │  │  │
│  │  └───────────────────────┼─────────────────────────────────────────┘  │  │
│  │                          │ RS232 (115200 8N1)                          │  │
│  │                          ▼                                             │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │                    RENDERER PROCESS (React)                      │  │  │
│  │  │  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐    │  │  │
│  │  │  │  Keypad   │  │  Status   │  │  Orders   │  │  Settings │    │  │  │
│  │  │  │    UI     │  │  Display  │  │  History  │  │   Panel   │    │  │  │
│  │  │  └───────────┘  └───────────┘  └───────────┘  └───────────┘    │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────────┘
                                       │
┌──────────────────────────────────────┼──────────────────────────────────────┐
│                          HARDWARE LAYER                                      │
│                          ┌───────────┴───────────┐                          │
│                          │    ECPay POS 终端     │                          │
│                          │   (RS232 / USB-COM)   │                          │
│                          └───────────────────────┘                          │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 数据流图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           TRANSACTION FLOW                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  User        Renderer       Main          Cloud         Go Server    POS    │
│   │            │             │              │              │          │     │
│   │  Click     │             │              │              │          │     │
│   │  "Pay"     │             │              │              │          │     │
│   ├───────────►│             │              │              │          │     │
│   │            │   IPC       │              │              │          │     │
│   │            │  invoke     │              │              │          │     │
│   │            ├────────────►│              │              │          │     │
│   │            │             │    HTTPS     │              │          │     │
│   │            │             │   Request    │              │          │     │
│   │            │             ├─────────────►│              │          │     │
│   │            │             │              │              │          │     │
│   │            │             │  Validate &  │              │          │     │
│   │            │             │  Return Token│              │          │     │
│   │            │             │◄─────────────┤              │          │     │
│   │            │             │              │              │          │     │
│   │            │             │  WebSocket   │              │          │     │
│   │            │             │   Command    │              │          │     │
│   │            │             ├──────────────┼─────────────►│          │     │
│   │            │             │              │              │          │     │
│   │            │             │              │              │  RS232   │     │
│   │            │             │              │              │  Frame   │     │
│   │            │             │              │              ├─────────►│     │
│   │            │             │              │              │          │     │
│   │            │             │              │              │   ACK    │     │
│   │            │             │              │              │◄─────────┤     │
│   │            │             │              │              │          │     │
│   │            │             │              │              │ (User    │     │
│   │            │             │              │              │  swipes  │     │
│   │            │             │              │              │  card)   │     │
│   │            │             │              │              │          │     │
│   │            │             │              │              │ Response │     │
│   │            │             │              │              │◄─────────┤     │
│   │            │             │              │              │          │     │
│   │            │             │  WebSocket   │              │          │     │
│   │            │             │   Response   │              │          │     │
│   │            │             │◄─────────────┼──────────────┤          │     │
│   │            │             │              │              │          │     │
│   │            │             │    HTTPS     │              │          │     │
│   │            │             │  Log Result  │              │          │     │
│   │            │             ├─────────────►│              │          │     │
│   │            │             │              │              │          │     │
│   │            │   IPC       │              │              │          │     │
│   │            │  Response   │              │              │          │     │
│   │            │◄────────────┤              │              │          │     │
│   │            │             │              │              │          │     │
│   │  Display   │             │              │              │          │     │
│   │  Result    │             │              │              │          │     │
│   │◄───────────┤             │              │              │          │     │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. 组件详细设计

### 3.1 Cloud Function

#### 3.1.1 职责

| 功能模块 | 描述 |
|----------|------|
| Auth Service | 设备注册、登录、Token 签发 |
| Credentials Vault | 安全存储 API Key、商户密钥等 |
| Business Logic | 交易验证、风控规则、业务处理 |
| Logs Collector | 接收并存储客户端日志 |
| Device Manager | 设备状态监控、远程配置下发 |

#### 3.1.2 API 设计

```yaml
# 设备认证
POST /api/v1/auth/device
Request:
  device_id: string      # 设备唯一标识 (硬件指纹)
  store_code: string     # 门店编号
  timestamp: number
  signature: string      # HMAC-SHA256(device_id + store_code + timestamp, device_secret)
Response:
  access_token: string   # JWT, 有效期 24h
  refresh_token: string  # 有效期 7d
  expires_in: number

# Token 刷新
POST /api/v1/auth/refresh
Request:
  refresh_token: string
Response:
  access_token: string
  expires_in: number

# 交易预处理 (获取交易所需的敏感参数)
POST /api/v1/transaction/prepare
Headers:
  Authorization: Bearer {access_token}
Request:
  trans_type: "SALE" | "REFUND" | "VOID" | "SETTLE"
  amount: string
  order_no?: string      # 退款/取消时需要
Response:
  transaction_id: string # 云端生成的交易流水号
  merchant_id: string    # 商户号
  terminal_id: string    # 终端号
  hash_key: string       # 用于本次交易的临时密钥
  expires_at: number     # 本次交易参数有效期 (5分钟)

# 交易结果上报
POST /api/v1/transaction/complete
Headers:
  Authorization: Bearer {access_token}
Request:
  transaction_id: string
  status: "SUCCESS" | "FAILED" | "TIMEOUT"
  pos_response: object   # POS 返回的原始数据
Response:
  recorded: boolean

# 日志上报
POST /api/v1/logs/batch
Headers:
  Authorization: Bearer {access_token}
Request:
  logs: Array<{
    level: "INFO" | "WARN" | "ERROR"
    timestamp: number
    message: string
    context?: object
  }>
Response:
  received: number
```

#### 3.1.3 安全机制

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         SECURITY LAYERS                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Layer 1: Transport Security                                                 │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  - TLS 1.3 强制                                                        │  │
│  │  - Certificate Pinning (可选，防中间人)                                │  │
│  │  - HSTS Header                                                         │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  Layer 2: Authentication                                                     │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  - Device Fingerprint (硬件唯一标识)                                   │  │
│  │  - Pre-shared Secret (设备注册时下发)                                  │  │
│  │  - JWT Token (短期有效)                                                │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  Layer 3: Authorization                                                      │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  - Role-based Access Control                                           │  │
│  │  - Transaction Limits (单笔/日累计)                                    │  │
│  │  - IP Whitelist (可选)                                                 │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  Layer 4: Audit                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  - 所有 API 调用记录                                                   │  │
│  │  - 异常行为检测                                                        │  │
│  │  - 实时告警                                                            │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### 3.2 Electron Main Process

#### 3.2.1 职责

| 模块 | 描述 |
|------|------|
| Cloud Client | 与 Cloud Function 通信，管理 Token |
| Process Manager | 管理 Go Server 子进程生命周期 |
| IPC Bridge | Renderer 与 Main 之间的通信桥梁 |
| Auto Updater | 应用自动更新 |
| Local Storage | 加密存储本地配置 |

#### 3.2.2 目录结构

```
electron-app/
├── package.json
├── electron-builder.yml
├── tsconfig.json
│
├── src/
│   ├── main/                      # Main Process
│   │   ├── index.ts               # 入口
│   │   ├── cloud-client.ts        # Cloud API 客户端
│   │   ├── process-manager.ts     # Go Server 进程管理
│   │   ├── ipc-handlers.ts        # IPC 处理器
│   │   ├── auto-updater.ts        # 自动更新
│   │   ├── device-id.ts           # 设备指纹生成
│   │   └── logger.ts              # 日志模块
│   │
│   ├── preload/                   # Preload Script
│   │   └── index.ts               # 暴露安全的 API 给 Renderer
│   │
│   └── renderer/                  # Renderer Process (React)
│       ├── index.html
│       ├── main.tsx
│       ├── App.tsx
│       ├── components/
│       ├── hooks/
│       └── ...
│
├── resources/
│   └── bin/
│       └── ecpay-server.exe       # Go 编译后的二进制
│
└── build/
    └── icon.ico
```

#### 3.2.3 核心代码

**process-manager.ts - Go Server 进程管理**

```typescript
import { spawn, ChildProcess } from 'child_process';
import { app } from 'electron';
import path from 'path';
import { EventEmitter } from 'events';

export class ProcessManager extends EventEmitter {
  private goServer: ChildProcess | null = null;
  private restartCount = 0;
  private maxRestarts = 5;
  private restartDelay = 3000;

  getServerPath(): string {
    if (app.isPackaged) {
      return path.join(process.resourcesPath, 'bin', 'ecpay-server.exe');
    }
    return path.join(__dirname, '../../resources/bin/ecpay-server.exe');
  }

  async start(): Promise<void> {
    if (this.goServer) {
      console.log('Go Server already running');
      return;
    }

    const serverPath = this.getServerPath();
    
    this.goServer = spawn(serverPath, [], {
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
      env: {
        ...process.env,
        ECPAY_LOG_DIR: path.join(app.getPath('userData'), 'logs'),
      },
    });

    this.goServer.stdout?.on('data', (data) => {
      const msg = data.toString().trim();
      console.log(`[Go] ${msg}`);
      this.emit('log', { level: 'INFO', message: msg });
    });

    this.goServer.stderr?.on('data', (data) => {
      const msg = data.toString().trim();
      console.error(`[Go Error] ${msg}`);
      this.emit('log', { level: 'ERROR', message: msg });
    });

    this.goServer.on('exit', (code, signal) => {
      console.log(`Go Server exited: code=${code}, signal=${signal}`);
      this.goServer = null;
      this.emit('exit', { code, signal });

      // 自动重启逻辑
      if (code !== 0 && this.restartCount < this.maxRestarts) {
        this.restartCount++;
        console.log(`Auto-restarting in ${this.restartDelay}ms (attempt ${this.restartCount})`);
        setTimeout(() => this.start(), this.restartDelay);
      }
    });

    this.goServer.on('error', (err) => {
      console.error('Failed to start Go Server:', err);
      this.emit('error', err);
    });

    // 等待服务就绪
    await this.waitForReady();
    this.restartCount = 0; // 成功启动后重置计数
    this.emit('ready');
  }

  private async waitForReady(timeout = 10000): Promise<void> {
    const start = Date.now();
    while (Date.now() - start < timeout) {
      try {
        const response = await fetch('http://localhost:8989/health');
        if (response.ok) return;
      } catch {
        // 服务还没准备好
      }
      await new Promise(r => setTimeout(r, 200));
    }
    throw new Error('Go Server failed to start within timeout');
  }

  stop(): void {
    if (this.goServer) {
      this.goServer.kill('SIGTERM');
      this.goServer = null;
    }
  }

  async restart(): Promise<void> {
    this.stop();
    await new Promise(r => setTimeout(r, 1000));
    await this.start();
  }

  isRunning(): boolean {
    return this.goServer !== null;
  }
}
```

**cloud-client.ts - Cloud API 客户端**

```typescript
import { safeStorage } from 'electron';
import Store from 'electron-store';

interface TokenData {
  accessToken: string;
  refreshToken: string;
  expiresAt: number;
}

export class CloudClient {
  private baseUrl: string;
  private store: Store;
  private tokenData: TokenData | null = null;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
    this.store = new Store({ name: 'cloud-auth' });
    this.loadTokens();
  }

  private loadTokens(): void {
    const encrypted = this.store.get('tokens') as Buffer | undefined;
    if (encrypted && safeStorage.isEncryptionAvailable()) {
      try {
        const decrypted = safeStorage.decryptString(encrypted);
        this.tokenData = JSON.parse(decrypted);
      } catch {
        this.tokenData = null;
      }
    }
  }

  private saveTokens(data: TokenData): void {
    this.tokenData = data;
    if (safeStorage.isEncryptionAvailable()) {
      const encrypted = safeStorage.encryptString(JSON.stringify(data));
      this.store.set('tokens', encrypted);
    }
  }

  async authenticate(deviceId: string, storeCode: string, deviceSecret: string): Promise<boolean> {
    const timestamp = Date.now();
    const signature = await this.sign(`${deviceId}${storeCode}${timestamp}`, deviceSecret);

    const response = await fetch(`${this.baseUrl}/api/v1/auth/device`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ device_id: deviceId, store_code: storeCode, timestamp, signature }),
    });

    if (!response.ok) return false;

    const data = await response.json();
    this.saveTokens({
      accessToken: data.access_token,
      refreshToken: data.refresh_token,
      expiresAt: Date.now() + data.expires_in * 1000,
    });
    return true;
  }

  async getAccessToken(): Promise<string | null> {
    if (!this.tokenData) return null;

    // Token 即将过期，刷新
    if (Date.now() > this.tokenData.expiresAt - 5 * 60 * 1000) {
      await this.refreshToken();
    }

    return this.tokenData?.accessToken || null;
  }

  private async refreshToken(): Promise<void> {
    if (!this.tokenData?.refreshToken) return;

    const response = await fetch(`${this.baseUrl}/api/v1/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: this.tokenData.refreshToken }),
    });

    if (response.ok) {
      const data = await response.json();
      this.saveTokens({
        ...this.tokenData,
        accessToken: data.access_token,
        expiresAt: Date.now() + data.expires_in * 1000,
      });
    }
  }

  async prepareTransaction(transType: string, amount: string, orderNo?: string): Promise<any> {
    const token = await this.getAccessToken();
    if (!token) throw new Error('Not authenticated');

    const response = await fetch(`${this.baseUrl}/api/v1/transaction/prepare`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      body: JSON.stringify({ trans_type: transType, amount, order_no: orderNo }),
    });

    if (!response.ok) throw new Error('Failed to prepare transaction');
    return response.json();
  }

  async completeTransaction(transactionId: string, status: string, posResponse: any): Promise<void> {
    const token = await this.getAccessToken();
    if (!token) return;

    await fetch(`${this.baseUrl}/api/v1/transaction/complete`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`,
      },
      body: JSON.stringify({ transaction_id: transactionId, status, pos_response: posResponse }),
    });
  }

  private async sign(data: string, secret: string): Promise<string> {
    const encoder = new TextEncoder();
    const key = await crypto.subtle.importKey(
      'raw',
      encoder.encode(secret),
      { name: 'HMAC', hash: 'SHA-256' },
      false,
      ['sign']
    );
    const signature = await crypto.subtle.sign('HMAC', key, encoder.encode(data));
    return Buffer.from(signature).toString('hex');
  }
}
```

**ipc-handlers.ts - IPC 处理器**

```typescript
import { ipcMain, BrowserWindow } from 'electron';
import { ProcessManager } from './process-manager';
import { CloudClient } from './cloud-client';
import WebSocket from 'ws';

export function setupIpcHandlers(
  processManager: ProcessManager,
  cloudClient: CloudClient,
  mainWindow: BrowserWindow
) {
  let wsConnection: WebSocket | null = null;

  // Go Server 控制
  ipcMain.handle('go-server:start', async () => {
    await processManager.start();
    return { success: true };
  });

  ipcMain.handle('go-server:stop', () => {
    processManager.stop();
    return { success: true };
  });

  ipcMain.handle('go-server:restart', async () => {
    await processManager.restart();
    return { success: true };
  });

  ipcMain.handle('go-server:status', () => {
    return { running: processManager.isRunning() };
  });

  // WebSocket 连接管理
  ipcMain.handle('ws:connect', () => {
    if (wsConnection) return { success: true, message: 'Already connected' };

    wsConnection = new WebSocket('ws://localhost:8989/ws');

    wsConnection.on('open', () => {
      mainWindow.webContents.send('ws:connected');
    });

    wsConnection.on('message', (data) => {
      mainWindow.webContents.send('ws:message', JSON.parse(data.toString()));
    });

    wsConnection.on('close', () => {
      wsConnection = null;
      mainWindow.webContents.send('ws:disconnected');
    });

    wsConnection.on('error', (err) => {
      mainWindow.webContents.send('ws:error', err.message);
    });

    return { success: true };
  });

  ipcMain.handle('ws:send', (_, message) => {
    if (!wsConnection || wsConnection.readyState !== WebSocket.OPEN) {
      return { success: false, error: 'Not connected' };
    }
    wsConnection.send(JSON.stringify(message));
    return { success: true };
  });

  ipcMain.handle('ws:disconnect', () => {
    if (wsConnection) {
      wsConnection.close();
      wsConnection = null;
    }
    return { success: true };
  });

  // Cloud API
  ipcMain.handle('cloud:authenticate', async (_, { deviceId, storeCode, deviceSecret }) => {
    return cloudClient.authenticate(deviceId, storeCode, deviceSecret);
  });

  ipcMain.handle('cloud:prepare-transaction', async (_, { transType, amount, orderNo }) => {
    return cloudClient.prepareTransaction(transType, amount, orderNo);
  });

  ipcMain.handle('cloud:complete-transaction', async (_, { transactionId, status, posResponse }) => {
    await cloudClient.completeTransaction(transactionId, status, posResponse);
    return { success: true };
  });
}
```

---

### 3.3 Go Server

#### 3.3.1 改动点

现有 Go Server 基本不需要大改，只需添加：

1. **Health Check 端点** - 供 Main Process 检测服务状态
2. **日志输出到 stdout** - 便于 Main Process 收集
3. **优雅关闭** - 响应 SIGTERM 信号

```go
// 添加到 main.go

import (
    "os"
    "os/signal"
    "syscall"
)

func main() {
    // ... 现有代码 ...

    // 添加 Health Check
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"status":"ok"}`))
    })

    // 优雅关闭
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-quit
        logger.Info("Shutting down server...")
        // 清理资源
        os.Exit(0)
    }()

    // ... 启动服务 ...
}
```

#### 3.3.2 编译配置

```bash
# Windows 64-bit
GOOS=windows GOARCH=amd64 go build \
  -ldflags="-s -w -H windowsgui" \
  -o ecpay-server.exe .

# 可选：UPX 压缩
upx --best --lzma ecpay-server.exe
```

编译参数说明：

- `-s`: 去除符号表
- `-w`: 去除 DWARF 调试信息
- `-H windowsgui`: 隐藏控制台窗口

---

### 3.4 Renderer Process (React)

#### 3.4.1 改动点

主要改动是将直接的 WebSocket 调用改为通过 IPC：

**preload/index.ts**

```typescript
import { contextBridge, ipcRenderer } from 'electron';

contextBridge.exposeInMainWorld('electronAPI', {
  // Go Server 控制
  goServer: {
    start: () => ipcRenderer.invoke('go-server:start'),
    stop: () => ipcRenderer.invoke('go-server:stop'),
    restart: () => ipcRenderer.invoke('go-server:restart'),
    status: () => ipcRenderer.invoke('go-server:status'),
  },

  // WebSocket
  ws: {
    connect: () => ipcRenderer.invoke('ws:connect'),
    send: (message: any) => ipcRenderer.invoke('ws:send', message),
    disconnect: () => ipcRenderer.invoke('ws:disconnect'),
    onConnected: (callback: () => void) => {
      ipcRenderer.on('ws:connected', callback);
      return () => ipcRenderer.removeListener('ws:connected', callback);
    },
    onDisconnected: (callback: () => void) => {
      ipcRenderer.on('ws:disconnected', callback);
      return () => ipcRenderer.removeListener('ws:disconnected', callback);
    },
    onMessage: (callback: (data: any) => void) => {
      const handler = (_: any, data: any) => callback(data);
      ipcRenderer.on('ws:message', handler);
      return () => ipcRenderer.removeListener('ws:message', handler);
    },
  },

  // Cloud API
  cloud: {
    authenticate: (params: any) => ipcRenderer.invoke('cloud:authenticate', params),
    prepareTransaction: (params: any) => ipcRenderer.invoke('cloud:prepare-transaction', params),
    completeTransaction: (params: any) => ipcRenderer.invoke('cloud:complete-transaction', params),
  },
});
```

**hooks/usePOS.ts (改造后)**

```typescript
import { useEffect, useCallback, useState } from 'react';

// 类型声明
declare global {
  interface Window {
    electronAPI: {
      goServer: {
        start: () => Promise<{ success: boolean }>;
        stop: () => Promise<{ success: boolean }>;
        restart: () => Promise<{ success: boolean }>;
        status: () => Promise<{ running: boolean }>;
      };
      ws: {
        connect: () => Promise<{ success: boolean }>;
        send: (message: any) => Promise<{ success: boolean }>;
        disconnect: () => Promise<{ success: boolean }>;
        onConnected: (callback: () => void) => () => void;
        onDisconnected: (callback: () => void) => () => void;
        onMessage: (callback: (data: any) => void) => () => void;
      };
      cloud: {
        prepareTransaction: (params: any) => Promise<any>;
        completeTransaction: (params: any) => Promise<{ success: boolean }>;
      };
    };
  }
}

export function usePOS(callbacks: POSCallbacks) {
  const [isConnected, setIsConnected] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);

  const addLog = useCallback((msg: string) => {
    const timestamp = new Date().toLocaleTimeString();
    setLogs((prev) => [...prev.slice(-49), `[${timestamp}] ${msg}`]);
  }, []);

  useEffect(() => {
    const { electronAPI } = window;

    // 监听 WebSocket 事件
    const unsubConnected = electronAPI.ws.onConnected(() => {
      setIsConnected(true);
      addLog('Connected to POS Server');
      callbacks.onConnect();
    });

    const unsubDisconnected = electronAPI.ws.onDisconnected(() => {
      setIsConnected(false);
      addLog('Disconnected from POS Server');
      callbacks.onDisconnect();
    });

    const unsubMessage = electronAPI.ws.onMessage((data) => {
      // 处理消息，与原来逻辑相同
      handleMessage(data);
    });

    // 启动连接
    electronAPI.ws.connect();

    return () => {
      unsubConnected();
      unsubDisconnected();
      unsubMessage();
    };
  }, []);

  const sendTransaction = useCallback(
    async (command: 'SALE' | 'REFUND', amount: string, orderNo?: string) => {
      const { electronAPI } = window;

      try {
        // 1. 从云端获取交易参数
        addLog(`Preparing ${command} transaction...`);
        const prepareData = await electronAPI.cloud.prepareTransaction({
          transType: command,
          amount,
          orderNo,
        });

        // 2. 发送到 Go Server
        addLog(`Sending ${command}: ${(parseInt(amount) / 100).toFixed(2)}`);
        await electronAPI.ws.send({
          command,
          amount,
          order_no: orderNo,
          transaction_id: prepareData.transaction_id,
          merchant_id: prepareData.merchant_id,
          terminal_id: prepareData.terminal_id,
        });

        return true;
      } catch (error) {
        addLog(`Error: ${error}`);
        return false;
      }
    },
    [addLog]
  );

  // ... 其他方法类似改造 ...

  return {
    isConnected,
    logs,
    sendTransaction,
    // ...
  };
}
```

---

## 4. 离线处理策略

### 4.1 离线检测

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         OFFLINE DETECTION                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Main Process                                                                │
│       │                                                                      │
│       ├─► 定时 Ping Cloud (每 30s)                                          │
│       │       │                                                              │
│       │       ├─► 成功: isOnline = true                                     │
│       │       │                                                              │
│       │       └─► 失败 (连续 3 次): isOnline = false                        │
│       │               │                                                      │
│       │               └─► 通知 Renderer 进入离线模式                        │
│       │                                                                      │
│       └─► 监听系统网络状态变化                                              │
│               │                                                              │
│               └─► navigator.onLine 变化时立即检测                           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 离线模式行为

| 功能 | 在线模式 | 离线模式 |
|------|----------|----------|
| 交易 (SALE/REFUND) | 正常 | 使用缓存的 Token，本地记录待同步 |
| 结算 (SETTLE) | 正常 | 禁用，提示需要网络 |
| 日志上报 | 实时 | 本地缓存，恢复后批量上传 |
| Token 刷新 | 自动 | 使用现有 Token 直到过期 |

### 4.3 数据同步

```typescript
// offline-queue.ts
interface PendingTransaction {
  id: string;
  timestamp: number;
  type: 'SALE' | 'REFUND';
  amount: string;
  posResponse: any;
}

class OfflineQueue {
  private store: Store;
  private queue: PendingTransaction[] = [];

  constructor() {
    this.store = new Store({ name: 'offline-queue' });
    this.queue = this.store.get('pending', []) as PendingTransaction[];
  }

  add(transaction: PendingTransaction): void {
    this.queue.push(transaction);
    this.store.set('pending', this.queue);
  }

  async syncAll(cloudClient: CloudClient): Promise<void> {
    const pending = [...this.queue];
    
    for (const tx of pending) {
      try {
        await cloudClient.completeTransaction(tx.id, 'SUCCESS', tx.posResponse);
        this.queue = this.queue.filter(t => t.id !== tx.id);
        this.store.set('pending', this.queue);
      } catch {
        // 同步失败，保留在队列中
        break;
      }
    }
  }

  getPendingCount(): number {
    return this.queue.length;
  }
}
```

---

## 5. 打包与部署

### 5.1 electron-builder 配置

```yaml
# electron-builder.yml
appId: com.yourcompany.ecpay-pos
productName: ECPay POS
copyright: Copyright © 2024 Your Company

directories:
  output: dist
  buildResources: build

files:
  - "dist/**/*"
  - "package.json"

extraResources:
  - from: resources/bin/
    to: bin/
    filter:
      - "**/*"

asar: true
asarUnpack:
  - "resources/bin/**"

win:
  target:
    - target: nsis
      arch:
        - x64
  icon: build/icon.ico
  artifactName: ${productName}-Setup-${version}.${ext}

nsis:
  oneClick: false
  allowToChangeInstallationDirectory: true
  installerIcon: build/icon.ico
  uninstallerIcon: build/icon.ico
  installerHeaderIcon: build/icon.ico
  createDesktopShortcut: true
  createStartMenuShortcut: true
  shortcutName: ECPay POS

publish:
  provider: generic
  url: https://your-update-server.com/releases/
```

### 5.2 构建脚本

```json
// package.json
{
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "build:electron": "npm run build && electron-builder",
    "build:go": "cd ../server && GOOS=windows GOARCH=amd64 go build -ldflags=\"-s -w\" -o ../electron-app/resources/bin/ecpay-server.exe .",
    "build:all": "npm run build:go && npm run build:electron"
  }
}
```

### 5.3 自动更新

```typescript
// auto-updater.ts
import { autoUpdater } from 'electron-updater';
import { BrowserWindow } from 'electron';

export function setupAutoUpdater(mainWindow: BrowserWindow) {
  autoUpdater.autoDownload = false;
  autoUpdater.autoInstallOnAppQuit = true;

  autoUpdater.on('update-available', (info) => {
    mainWindow.webContents.send('update:available', info);
  });

  autoUpdater.on('download-progress', (progress) => {
    mainWindow.webContents.send('update:progress', progress);
  });

  autoUpdater.on('update-downloaded', () => {
    mainWindow.webContents.send('update:ready');
  });

  // 每小时检查一次更新
  setInterval(() => {
    autoUpdater.checkForUpdates();
  }, 60 * 60 * 1000);

  // 启动时检查
  autoUpdater.checkForUpdates();
}
```

---

## 6. 安全加固

### 6.1 Electron 安全配置

```typescript
// main/index.ts
const mainWindow = new BrowserWindow({
  webPreferences: {
    nodeIntegration: false,           // 禁用 Node.js 集成
    contextIsolation: true,           // 启用上下文隔离
    sandbox: true,                    // 启用沙箱
    preload: path.join(__dirname, 'preload.js'),
  },
});

// 禁用远程模块
app.on('remote-require', (event) => event.preventDefault());
app.on('remote-get-builtin', (event) => event.preventDefault());
app.on('remote-get-global', (event) => event.preventDefault());
app.on('remote-get-current-window', (event) => event.preventDefault());
app.on('remote-get-current-web-contents', (event) => event.preventDefault());
```

### 6.2 Go Server 安全

```go
// 只监听 localhost
http.ListenAndServe("127.0.0.1:8989", nil)

// 验证请求来源 (可选)
func validateOrigin(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    return origin == "" || origin == "http://localhost:5173"
}
```

### 6.3 敏感数据处理

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      SENSITIVE DATA HANDLING                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  数据类型              存储位置           加密方式                           │
│  ─────────────────────────────────────────────────────────────────────────  │
│  API Key               Cloud Only         N/A (不离开云端)                   │
│  Merchant Secret       Cloud Only         N/A (不离开云端)                   │
│  Access Token          Main Process       Electron safeStorage              │
│  Refresh Token         Main Process       Electron safeStorage              │
│  Device Secret         首次注册时下发     safeStorage + 硬件绑定            │
│  交易日志              本地 + 云端        AES-256 (本地)                     │
│  卡号                  不存储             仅显示掩码 (****1234)              │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 7. 监控与运维

### 7.1 健康检查

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         HEALTH CHECK FLOW                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Main Process (每 10s)                                                       │
│       │                                                                      │
│       ├─► Check Go Server: GET http://localhost:8989/health                 │
│       │       │                                                              │
│       │       ├─► OK: goServerStatus = 'running'                            │
│       │       │                                                              │
│       │       └─► Fail: goServerStatus = 'error', 尝试重启                  │
│       │                                                                      │
│       ├─► Check Cloud: GET https://api.example.com/health                   │
│       │       │                                                              │
│       │       ├─► OK: cloudStatus = 'connected'                             │
│       │       │                                                              │
│       │       └─► Fail: cloudStatus = 'offline'                             │
│       │                                                                      │
│       └─► 汇总状态发送给 Renderer                                           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 日志架构

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           LOG ARCHITECTURE                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                   │
│  │  Renderer   │     │    Main     │     │  Go Server  │                   │
│  │   Logs      │     │   Logs      │     │    Logs     │                   │
│  └──────┬──────┘     └──────┬──────┘     └──────┬──────┘                   │
│         │                   │                   │                           │
│         └───────────────────┼───────────────────┘                           │
│                             │                                               │
│                             ▼                                               │
│                    ┌─────────────────┐                                      │
│                    │  Log Aggregator │                                      │
│                    │  (Main Process) │                                      │
│                    └────────┬────────┘                                      │
│                             │                                               │
│              ┌──────────────┼──────────────┐                               │
│              │              │              │                               │
│              ▼              ▼              ▼                               │
│       ┌───────────┐  ┌───────────┐  ┌───────────┐                         │
│       │  Console  │  │ Local File│  │   Cloud   │                         │
│       │  (Dev)    │  │ (Rotate)  │  │  (Batch)  │                         │
│       └───────────┘  └───────────┘  └───────────┘                         │
│                                                                              │
│  本地日志路径: %APPDATA%/ecpay-pos/logs/                                    │
│  保留策略: 最近 7 天，单文件最大 10MB                                       │
│  云端上报: 每 5 分钟批量上传，或累积 100 条                                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 7.3 错误上报

```typescript
// error-reporter.ts
interface ErrorReport {
  timestamp: number;
  type: 'crash' | 'error' | 'warning';
  message: string;
  stack?: string;
  context: {
    version: string;
    platform: string;
    goServerStatus: string;
    lastTransaction?: string;
  };
}

class ErrorReporter {
  async report(error: ErrorReport): Promise<void> {
    // 本地记录
    this.logToFile(error);

    // 云端上报 (非阻塞)
    try {
      await fetch(`${CLOUD_URL}/api/v1/errors`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(error),
      });
    } catch {
      // 忽略上报失败
    }
  }
}
```

---

## 8. 实施计划

### 8.1 阶段划分

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        IMPLEMENTATION PHASES                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Phase 1: 基础框架 (1-2 周)                                                  │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  □ 初始化 Electron 项目                                                │  │
│  │  □ 迁移现有 React 代码到 Renderer                                      │  │
│  │  □ 实现 Go Server 进程管理                                             │  │
│  │  □ 实现 IPC 通信层                                                     │  │
│  │  □ 本地测试通过                                                        │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  Phase 2: Cloud 集成 (1-2 周)                                                │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  □ 部署 Cloud Function                                                 │  │
│  │  □ 实现设备认证流程                                                    │  │
│  │  □ 实现交易预处理 API                                                  │  │
│  │  □ 实现日志上报                                                        │  │
│  │  □ 联调测试                                                            │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  Phase 3: 离线支持 (1 周)                                                    │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  □ 实现离线检测                                                        │  │
│  │  □ 实现离线队列                                                        │  │
│  │  □ 实现数据同步                                                        │  │
│  │  □ 离线场景测试                                                        │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  Phase 4: 打包部署 (1 周)                                                    │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │  □ 配置 electron-builder                                               │  │
│  │  □ 实现自动更新                                                        │  │
│  │  □ 代码签名 (可选)                                                     │  │
│  │  □ 生成安装包                                                          │  │
│  │  □ 用户验收测试                                                        │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 8.2 技术栈

| 组件 | 技术选型 | 版本 |
|------|----------|------|
| Electron | electron | ^28.0.0 |
| 构建工具 | electron-builder | ^24.0.0 |
| 前端框架 | React + TypeScript | ^18.0.0 |
| 打包工具 | Vite | ^5.0.0 |
| 状态管理 | React Hooks | - |
| 本地存储 | electron-store | ^8.0.0 |
| Go Server | Go | 1.21+ |
| Cloud | AWS Lambda / Cloudflare Workers | - |

### 8.3 文件迁移清单

```
现有项目                          Electron 项目
─────────────────────────────────────────────────────────────────
webapp/src/App.tsx           →   electron-app/src/renderer/App.tsx
webapp/src/components/*      →   electron-app/src/renderer/components/*
webapp/src/hooks/*           →   electron-app/src/renderer/hooks/* (需改造)
server/                      →   保持独立，编译后复制到 resources/bin/
```

---

## 9. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| Electron 安装包过大 | 用户下载慢 | 使用 NSIS 压缩，提供增量更新 |
| Go Server 崩溃 | 交易中断 | 自动重启 + 状态恢复 |
| 网络不稳定 | 云端不可达 | 离线模式 + 本地缓存 |
| Token 过期 | 认证失败 | 自动刷新 + 离线 Token 缓存 |
| 设备被克隆 | 安全风险 | 硬件指纹绑定 + 异常检测 |

---

## 10. 附录

### 10.1 设备指纹生成

```typescript
// device-id.ts
import { machineIdSync } from 'node-machine-id';
import { createHash } from 'crypto';
import os from 'os';

export function generateDeviceId(): string {
  const machineId = machineIdSync();
  const cpuInfo = os.cpus()[0]?.model || '';
  const hostname = os.hostname();
  
  const raw = `${machineId}:${cpuInfo}:${hostname}`;
  return createHash('sha256').update(raw).digest('hex').substring(0, 32);
}
```

### 10.2 完整 Main Process 入口

```typescript
// main/index.ts
import { app, BrowserWindow, ipcMain } from 'electron';
import path from 'path';
import { ProcessManager } from './process-manager';
import { CloudClient } from './cloud-client';
import { setupIpcHandlers } from './ipc-handlers';
import { setupAutoUpdater } from './auto-updater';
import { generateDeviceId } from './device-id';

const CLOUD_URL = process.env.CLOUD_URL || 'https://api.your-domain.com';

let mainWindow: BrowserWindow | null = null;
const processManager = new ProcessManager();
const cloudClient = new CloudClient(CLOUD_URL);

async function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: true,
      preload: path.join(__dirname, 'preload.js'),
    },
  });

  if (process.env.NODE_ENV === 'development') {
    mainWindow.loadURL('http://localhost:5173');
    mainWindow.webContents.openDevTools();
  } else {
    mainWindow.loadFile(path.join(__dirname, '../renderer/index.html'));
  }

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

app.whenReady().then(async () => {
  // 1. 启动 Go Server
  await processManager.start();

  // 2. 创建窗口
  await createWindow();

  // 3. 设置 IPC 处理器
  setupIpcHandlers(processManager, cloudClient, mainWindow!);

  // 4. 设置自动更新
  if (app.isPackaged) {
    setupAutoUpdater(mainWindow!);
  }

  // 5. 暴露设备 ID
  ipcMain.handle('get-device-id', () => generateDeviceId());
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('before-quit', () => {
  processManager.stop();
});

app.on('activate', () => {
  if (mainWindow === null) {
    createWindow();
  }
});
```

---

## 11. 总结

本方案通过 Electron + Go Server + Cloud Function 的混合架构，实现了：

1. **安全性**：敏感 credentials 永不离开云端
2. **可靠性**：支持离线降级，自动恢复
3. **易用性**：单一安装包，开箱即用
4. **可维护性**：远程更新，集中日志

相比纯本地方案，增加了 Cloud Function 和 Electron 封装的复杂度，但换来了企业级的安全保障和运维能力。

---

## 12. 详细迁移计划

### 12.1 迁移路线图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         MIGRATION ROADMAP                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Week 1                Week 2                Week 3                Week 4    │
│    │                     │                     │                     │       │
│    ▼                     ▼                     ▼                     ▼       │
│  ┌─────┐              ┌─────┐              ┌─────┐              ┌─────┐     │
│  │ P1  │──────────────│ P2  │──────────────│ P3  │──────────────│ P4  │     │
│  │基础 │              │云端 │              │离线 │              │部署 │     │
│  │框架 │              │集成 │              │支持 │              │发布 │     │
│  └─────┘              └─────┘              └─────┘              └─────┘     │
│    │                     │                     │                     │       │
│    ├─ Electron 初始化    ├─ Cloud Function     ├─ 离线检测           ├─ 打包  │
│    ├─ React 迁移         ├─ 设备认证           ├─ 离线队列           ├─ 签名  │
│    ├─ Go 进程管理        ├─ 交易 API           ├─ 数据同步           ├─ 更新  │
│    └─ IPC 通信           └─ 日志上报           └─ 测试               └─ UAT   │
│                                                                              │
│  ════════════════════════════════════════════════════════════════════════   │
│  里程碑:                                                                     │
│  M1: 本地 Electron 可运行 (Week 1 End)                                       │
│  M2: 云端联调通过 (Week 2 End)                                               │
│  M3: 离线功能完成 (Week 3 End)                                               │
│  M4: 生产就绪 (Week 4 End)                                                   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 12.2 Phase 1: 基础框架

#### 12.2.1 任务分解

| 任务 | 描述 | 预估工时 | 依赖 |
|------|------|----------|------|
| T1.1 | 初始化 Electron 项目 (Vite + React + TS) | 4h | - |
| T1.2 | 配置 electron-builder | 2h | T1.1 |
| T1.3 | 迁移 React 组件 (App, Keypad, OrderHistory, ServerStatus) | 4h | T1.1 |
| T1.4 | 迁移 CSS/Tailwind 配置 | 2h | T1.3 |
| T1.5 | 实现 ProcessManager (Go Server 进程管理) | 6h | T1.1 |
| T1.6 | 实现 Preload Script | 3h | T1.1 |
| T1.7 | 实现 IPC Handlers (WebSocket 桥接) | 6h | T1.5, T1.6 |
| T1.8 | 改造 usePOS Hook (IPC 版本) | 4h | T1.7 |
| T1.9 | 改造 useAppState Hook | 2h | T1.8 |
| T1.10 | Go Server 添加 /health 端点 | 1h | - |
| T1.11 | Go Server 添加优雅关闭 | 1h | T1.10 |
| T1.12 | 本地集成测试 | 4h | T1.8, T1.11 |
| T1.13 | Bug 修复 & 调优 | 4h | T1.12 |

**Phase 1 总计: 43h ≈ 5-6 工作日**

#### 12.2.2 文件变更清单

```
新建文件:
─────────────────────────────────────────────────────────────────
electron-app/
├── package.json                    # 新建
├── electron-builder.yml            # 新建
├── tsconfig.json                   # 新建
├── vite.config.ts                  # 新建 (Electron 适配)
├── src/
│   ├── main/
│   │   ├── index.ts                # 新建 - Main Process 入口
│   │   ├── process-manager.ts      # 新建 - Go 进程管理
│   │   ├── ipc-handlers.ts         # 新建 - IPC 处理器
│   │   └── logger.ts               # 新建 - 日志模块
│   └── preload/
│       └── index.ts                # 新建 - Preload Script

迁移文件 (从 webapp/):
─────────────────────────────────────────────────────────────────
webapp/src/App.tsx              →  electron-app/src/renderer/App.tsx
webapp/src/App.css              →  electron-app/src/renderer/App.css
webapp/src/index.css            →  electron-app/src/renderer/index.css
webapp/src/main.tsx             →  electron-app/src/renderer/main.tsx
webapp/src/components/*         →  electron-app/src/renderer/components/*
webapp/src/hooks/useAppState.ts →  electron-app/src/renderer/hooks/useAppState.ts
webapp/src/hooks/useOrders.ts   →  electron-app/src/renderer/hooks/useOrders.ts
webapp/index.html               →  electron-app/src/renderer/index.html

需要改造的文件:
─────────────────────────────────────────────────────────────────
webapp/src/hooks/usePOS.ts      →  electron-app/src/renderer/hooks/usePOS.ts
                                   (WebSocket → IPC)

Go Server 改动:
─────────────────────────────────────────────────────────────────
server/main.go                     # 添加 /health 端点 + 优雅关闭
```

#### 12.2.3 验收标准

- [ ] `npm run dev` 可启动 Electron 开发环境
- [ ] Go Server 作为子进程自动启动
- [ ] React UI 正常显示
- [ ] WebSocket 通过 IPC 正常通信
- [ ] 交易流程 (SALE/REFUND) 正常工作
- [ ] Go Server 崩溃后自动重启

---

### 12.3 Phase 2: Cloud 集成

#### 12.3.1 任务分解

| 任务 | 描述 | 预估工时 | 依赖 |
|------|------|----------|------|
| T2.1 | 设计 Cloud Function 数据库 Schema | 2h | - |
| T2.2 | 实现 /api/v1/auth/device (设备认证) | 4h | T2.1 |
| T2.3 | 实现 /api/v1/auth/refresh (Token 刷新) | 2h | T2.2 |
| T2.4 | 实现 /api/v1/transaction/prepare | 4h | T2.2 |
| T2.5 | 实现 /api/v1/transaction/complete | 3h | T2.4 |
| T2.6 | 实现 /api/v1/logs/batch | 2h | T2.2 |
| T2.7 | 部署 Cloud Function (AWS/Cloudflare) | 3h | T2.2-T2.6 |
| T2.8 | 实现 CloudClient (Electron Main) | 6h | T2.7 |
| T2.9 | 实现设备指纹生成 | 2h | - |
| T2.10 | 实现 Token 加密存储 (safeStorage) | 3h | T2.8 |
| T2.11 | 添加 Cloud IPC Handlers | 3h | T2.8 |
| T2.12 | 改造 usePOS 集成 Cloud API | 4h | T2.11 |
| T2.13 | 添加设备注册/登录 UI | 4h | T2.12 |
| T2.14 | 联调测试 | 4h | T2.13 |
| T2.15 | Bug 修复 | 4h | T2.14 |

**Phase 2 总计: 50h ≈ 6-7 工作日**

#### 12.3.2 Cloud Function 技术选型

| 选项 | 优点 | 缺点 | 推荐场景 |
|------|------|------|----------|
| AWS Lambda + API Gateway | 成熟稳定，生态完善 | 冷启动延迟，成本较高 | 企业级，已有 AWS 基础设施 |
| Cloudflare Workers | 边缘部署，延迟低，免费额度大 | 运行时限制 (CPU 10ms) | 轻量级，全球分布 |
| Vercel Serverless | 开发体验好，与 Next.js 集成 | 免费额度有限 | 快速原型 |
| 自建 (Go/Node) | 完全控制 | 需要运维 | 有运维能力的团队 |

**推荐: Cloudflare Workers** (成本低，延迟低，足够简单的业务逻辑)

#### 12.3.3 验收标准

- [ ] 设备可以注册并获取 Token
- [ ] Token 自动刷新正常
- [ ] 交易前从云端获取参数
- [ ] 交易结果上报云端
- [ ] 日志批量上报正常
- [ ] Token 加密存储在本地

---

### 12.4 Phase 3: 离线支持 (3-4 工作日)

#### 12.4.1 任务分解

| 任务 | 描述 | 预估工时 | 依赖 |
|------|------|----------|------|
| T3.1 | 实现网络状态检测 | 3h | - |
| T3.2 | 实现 OfflineQueue 类 | 4h | - |
| T3.3 | 实现离线交易逻辑 | 4h | T3.2 |
| T3.4 | 实现网络恢复后自动同步 | 3h | T3.2, T3.3 |
| T3.5 | 添加离线状态 UI 提示 | 2h | T3.1 |
| T3.6 | 添加待同步数量显示 | 1h | T3.2 |
| T3.7 | 离线场景测试 | 4h | T3.1-T3.6 |
| T3.8 | 边界情况处理 | 3h | T3.7 |

**Phase 3 总计: 24h ≈ 3-4 工作日**

#### 12.4.2 离线场景测试用例

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         OFFLINE TEST CASES                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  TC-3.1: 启动时离线                                                          │
│  ─────────────────────────────────────────────────────────────────────────  │
│  前置: 断开网络                                                              │
│  步骤: 启动应用                                                              │
│  预期: 显示离线提示，使用缓存 Token，可进行交易                              │
│                                                                              │
│  TC-3.2: 交易中断网                                                          │
│  ─────────────────────────────────────────────────────────────────────────  │
│  前置: 应用在线运行                                                          │
│  步骤: 发起交易 → 交易过程中断网 → 交易完成                                  │
│  预期: 交易正常完成，结果存入离线队列                                        │
│                                                                              │
│  TC-3.3: 网络恢复同步                                                        │
│  ─────────────────────────────────────────────────────────────────────────  │
│  前置: 离线队列有待同步数据                                                  │
│  步骤: 恢复网络                                                              │
│  预期: 自动同步，队列清空，UI 更新                                           │
│                                                                              │
│  TC-3.4: Token 过期离线                                                      │
│  ─────────────────────────────────────────────────────────────────────────  │
│  前置: Token 即将过期，断开网络                                              │
│  步骤: 等待 Token 过期 → 尝试交易                                            │
│  预期: 提示需要联网重新认证                                                  │
│                                                                              │
│  TC-3.5: 结算离线阻止                                                        │
│  ─────────────────────────────────────────────────────────────────────────  │
│  前置: 离线状态                                                              │
│  步骤: 尝试结算操作                                                          │
│  预期: 阻止操作，提示需要网络                                                │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### 12.4.3 验收标准

- [ ] 离线状态正确检测和显示
- [ ] 离线时可完成交易
- [ ] 交易结果存入本地队列
- [ ] 网络恢复后自动同步
- [ ] 同步失败有重试机制
- [ ] 结算操作离线时被阻止

---

### 12.5 Phase 4: 打包部署 (3-4 工作日)

#### 12.5.1 任务分解

| 任务 | 描述 | 预估工时 | 依赖 |
|------|------|----------|------|
| T4.1 | 配置 electron-builder (NSIS) | 3h | - |
| T4.2 | 配置 Go 交叉编译脚本 | 2h | - |
| T4.3 | 集成 Go 二进制到打包流程 | 2h | T4.1, T4.2 |
| T4.4 | 实现自动更新 (electron-updater) | 4h | T4.1 |
| T4.5 | 搭建更新服务器 | 2h | T4.4 |
| T4.6 | 代码签名配置 (可选) | 4h | T4.1 |
| T4.7 | 生成测试安装包 | 2h | T4.3 |
| T4.8 | Windows 环境测试 | 4h | T4.7 |
| T4.9 | 用户验收测试 (UAT) | 4h | T4.8 |
| T4.10 | 文档编写 (安装/使用指南) | 3h | T4.9 |

**Phase 4 总计: 30h ≈ 4 工作日**

#### 12.5.2 验收标准

- [ ] 安装包可在 Windows 10/11 正常安装
- [ ] 安装后应用可正常启动
- [ ] Go Server 随应用启动
- [ ] 自动更新检测正常
- [ ] 更新下载和安装正常
- [ ] 卸载干净无残留

---

## 13. 工作量评估汇总

### 13.1 总体工时

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         EFFORT ESTIMATION                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Phase              Tasks    Hours    Days (8h)    Buffer (20%)    Total    │
│  ─────────────────────────────────────────────────────────────────────────  │
│  P1: 基础框架        13       43h       5.4d          1.1d          6.5d    │
│  P2: Cloud 集成      15       50h       6.3d          1.3d          7.6d    │
│  P3: 离线支持         8       24h       3.0d          0.6d          3.6d    │
│  P4: 打包部署        10       30h       3.8d          0.8d          4.6d    │
│  ─────────────────────────────────────────────────────────────────────────  │
│  TOTAL              46      147h      18.5d          3.8d         22.3d    │
│                                                                              │
│  ════════════════════════════════════════════════════════════════════════   │
│                                                                              │
│  预估总工期: 22-25 工作日 (约 4.5-5 周)                                      │
│                                                                              │
│  假设条件:                                                                   │
│  - 1 名全栈开发者 (熟悉 React + Go + Electron)                              │
│  - 每天有效工作 8 小时                                                       │
│  - 无重大技术障碍                                                            │
│  - Cloud Function 选用 Cloudflare Workers (学习成本低)                       │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```
