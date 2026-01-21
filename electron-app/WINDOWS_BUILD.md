# Windows 构建指南

## 快速开始

```bash
cd electron-app
npm run build:win
```

输出文件：`release/ECPay POS Setup 1.0.0.exe`（~158 MB）

## 构建命令

| 命令 | 说明 |
|------|------|
| `npm run build:win` | 默认构建（x64，生产优化） |
| `npm run build:win:debug` | 调试模式（保留源码映射） |
| `npm run build:win:arm64` | ARM64 架构 |
| `npm run build:win:all` | 同时构建 x64 和 ARM64 |
| `npm run build:win:clean` | 清理后重新构建 |

## Shell 脚本

```bash
# macOS/Linux
chmod +x build-windows.sh
./build-windows.sh [--debug] [--arm64] [--all] [--clean]

# Windows
build-windows.bat [--debug] [--arm64] [--all] [--clean]
```

## 依赖打包

✅ **完全自包含** - Windows 用户无需安装任何依赖

包含：
- Electron 运行时（Chromium + Node.js）
- 所有 Node.js 依赖
- Go 服务器（ecpay-server.exe）
- 所有资源文件

## 架构支持

| 架构 | 兼容性 | 推荐 |
|------|--------|------|
| x64 | Windows 7+ (Intel/AMD) | ✅ |
| ARM64 | Windows 11 ARM | - |

## 系统要求

- Windows 7 SP1 或更高版本
- 500 MB 磁盘空间
- 无需 .NET Framework 或 Visual C++ 运行时

## 故障排除

**构建失败**：运行 `npm run build:win:clean`

**安装程序无法运行**：
1. 检查 Windows 版本 >= 7 SP1
2. 检查磁盘空间
3. 以管理员身份运行

**Go 服务器无法启动**：
1. 检查串口连接
2. 查看日志：`%APPDATA%\ECPay POS\logs\`
