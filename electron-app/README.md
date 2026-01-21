# ECPay POS - Electron Application

基于 Electron 封装的 ECPay POS 终端应用，集成 Go Server 和 React UI。

## 架构概览

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Electron Application                              │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                     Main Process (Node.js)                         │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐   │  │
│  │  │   Process   │  │  WebSocket  │  │      IPC Handlers       │   │  │
│  │  │   Manager   │  │   Bridge    │  │                         │   │  │
│  │  └──────┬──────┘  └──────┬──────┘  └───────────┬─────────────┘   │  │
│  │         │                │                     │                  │  │
│  │         │ spawn          │ ws://localhost:8989 │ IPC              │  │
│  │         ▼                ▼                     ▼                  │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐   │  │
│  │  │  Go Server  │  │  Go Server  │  │   Renderer Process      │   │  │
│  │  │  (Child)    │◄─┤  WebSocket  │  │   (React + Preload)     │   │  │
│  │  └──────┬──────┘  └─────────────┘  └─────────────────────────┘   │  │
│  │         │                                                         │  │
│  └─────────┼─────────────────────────────────────────────────────────┘  │
│            │ RS232                                                       │
│            ▼                                                             │
│     ┌─────────────┐                                                     │
│     │  POS 终端   │                                                     │
│     └─────────────┘                                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

## 项目结构

```
electron-app/
├── src/
│   ├── main/                    # Main Process
│   │   ├── index.ts             # 应用入口
│   │   ├── config.ts            # 配置常量
│   │   ├── logger.ts            # 日志模块
│   │   ├── types.ts             # 类型定义
│   │   ├── process-manager.ts   # Go Server 进程管理
│   │   ├── websocket-bridge.ts  # WebSocket 桥接
│   │   ├── ipc-handlers.ts      # IPC 处理器
│   │   └── preload.ts           # Preload Script
│   │
│   └── renderer/                # Renderer Process
│       ├── App.tsx              # 主组件
│       ├── main.tsx             # React 入口
│       ├── index.html           # HTML 模板
│       ├── index.css            # 全局样式
│       ├── components/          # UI 组件
│       │   ├── Keypad.tsx
│       │   ├── OrderHistory.tsx
│       │   └── ServerStatus.tsx
│       └── hooks/               # React Hooks
│           ├── useAppState.ts   # 状态机
│           ├── useOrders.ts     # 订单管理
│           └── usePOS.ts        # POS 通信
│
├── resources/
│   └── bin/                     # Go Server 二进制
│       └── ecpay-server[.exe]
│
├── scripts/
│   └── dev.js                   # 开发启动脚本
│
├── package.json
├── tsconfig.main.json           # Main Process TS 配置
├── tsconfig.json                # Renderer TS 配置
├── vite.config.ts               # Vite 配置
├── tailwind.config.js           # Tailwind 配置
└── electron-builder.yml         # 打包配置 (可选)
```

## 快速开始

### 1. 安装依赖

```bash
npm install
```

### 2. 编译 Go Server

```bash
# macOS / Linux
npm run build:go:mac

# Windows (交叉编译)
npm run build:go:win
```

### 3. 开发模式

**方式一：使用开发脚本（推荐）**

```bash
npm run dev
```

**方式二：手动启动各组件**

```bash
# 终端 1: 编译 Main Process
npm run build:main

# 终端 2: 启动 Vite 开发服务器
npm run dev:renderer

# 终端 3: 启动 Electron
npm run start
```

### 4. 生产构建

```bash
# 构建所有组件
npm run build

# 打包为安装程序
npm run dist
```

## 开发说明

### Main Process

Main Process 负责：
- 管理 Go Server 子进程生命周期
- 维护与 Go Server 的 WebSocket 连接
- 通过 IPC 与 Renderer 通信
- 处理应用窗口和系统事件

关键模块：
- `ProcessManager`: 管理 Go Server 进程，支持自动重启
- `WebSocketBridge`: 桥接 Go Server WebSocket 和 Renderer IPC
- `IpcHandlers`: 定义所有 IPC 通道和处理逻辑

### Renderer Process

Renderer Process 是 React 应用，运行在沙箱环境中：
- 只能通过 `window.electronAPI` 访问系统功能
- 所有敏感操作都通过 IPC 委托给 Main Process

### Preload Script

Preload Script 定义了暴露给 Renderer 的安全 API：

```typescript
window.electronAPI = {
  goServer: {
    start, stop, restart, status, onLog
  },
  ws: {
    connect, send, disconnect, status,
    onConnected, onDisconnected, onMessage, onError
  },
  app: {
    platform, isPackaged
  }
}
```

### 浏览器开发模式

在没有 Electron 的情况下（直接用浏览器访问 Vite 开发服务器），
`usePOS` hook 会自动回退到直接 WebSocket 连接模式。

这允许在不启动 Electron 的情况下开发和调试 UI。

## 配置

配置项在 `src/main/config.ts` 中定义：

```typescript
export const config = {
  goServer: {
    port: 8989,
    host: '127.0.0.1',
    maxRestarts: 5,
    restartDelay: 3000,
    startupTimeout: 10000,
  },
  websocket: {
    reconnectDelay: 3000,
    url: 'ws://127.0.0.1:8989/ws',
  },
  window: {
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
  },
  devServer: {
    url: 'http://localhost:5173',
  },
};
```

## 安全性

本应用遵循 Electron 安全最佳实践：

- ✅ `nodeIntegration: false` - 禁用 Node.js 集成
- ✅ `contextIsolation: true` - 启用上下文隔离
- ✅ `sandbox: true` - 启用沙箱模式
- ✅ 使用 `contextBridge` 暴露有限 API
- ✅ 阻止新窗口创建
- ✅ Go Server 只监听 localhost

## 打包

### 开发打包（不压缩）

```bash
npm run pack
```

### 生产打包

```bash
npm run dist
```

输出目录：`release/`

### 打包配置

打包配置在 `package.json` 的 `build` 字段中定义，
或者可以创建独立的 `electron-builder.yml` 文件。

## 故障排除

### Go Server 启动失败

1. 检查 `resources/bin/ecpay-server` 是否存在
2. 检查文件是否有执行权限：`chmod +x resources/bin/ecpay-server`
3. 查看控制台日志获取详细错误信息

### WebSocket 连接失败

1. 确认 Go Server 已启动（查看日志）
2. 检查端口 8989 是否被占用
3. 尝试重启 Go Server

### Vite 开发服务器问题

1. 确保端口 5173 未被占用
2. 检查 `postcss.config.js` 和 `tailwind.config.js` 语法

## 许可证

MIT
