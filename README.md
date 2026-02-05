# TCB POS Server

A WebSocket-based gateway server that bridges web applications with TCB POS terminals via RS232 serial communication.

## Architecture

### Development Mode (Web)

```
┌─────────────┐    WebSocket    ┌──────────────┐    TCP/RS232    ┌──────────────┐
│   Webapp    │ ◄───:5173───►   │  Server_TCB  │ ◄───:9999───►   │  Mock POS    │
│  (React)    │                 │    (Go)      │                 │   (Go)       │
└─────────────┘                 └──────────────┘                 └──────────────┘
```

### Production Mode (Electron)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Electron Application                                │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  Main Process                                                        │   │
│  │  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐          │   │
│  │  │   Process    │    │   WebSocket  │    │     IPC      │          │   │
│  │  │   Manager    │    │    Bridge    │    │   Handlers   │          │   │
│  │  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘          │   │
│  │         │ spawn             │ ws                │ ipc              │   │
│  │         ▼                   ▼                   ▼                  │   │
│  │  ┌──────────────┐    ┌──────────────────────────────────┐         │   │
│  │  │  Go Server   │◄──►│        Renderer (React UI)       │         │   │
│  │  └──────┬───────┘    └──────────────────────────────────┘         │   │
│  └─────────┼────────────────────────────────────────────────────────────┘   │
│            │ RS232                                                           │
│            ▼                                                                 │
│     ┌─────────────┐                                                         │
│     │  POS 终端   │                                                         │
│     └─────────────┘                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Components

| Component | Directory | Port | Description |
|-----------|-----------|------|-------------|
| **Server_TCB** | `server_tcb/` | `:8989` | Go WebSocket server bridging webapp to POS |
| **Mock POS (TCB)** | `mock-pos-tcb/` | `:9999` | POS terminal simulator for development |
| **Webapp** | `webapp/` | `:5173` | React TypeScript frontend for POS operations |
| **Electron App** | `electron-app/` | - | Desktop application bundling Server_TCB + Webapp |

## Quick Start

### Option 1: Web Development (Mock Mode)

```bash
# 1. Start Mock POS (in terminal 1)
cd mock-pos-tcb && go run main.go

# 2. Start Server_TCB (in terminal 2)
cd server_tcb && go run main.go

# 3. Start Webapp (in terminal 3)
cd webapp && npm install && npm run dev
```

Open <http://localhost:5173> in your browser.

### Option 2: Electron Application

```bash
# 1. Install dependencies
cd electron-app && npm install

# 2. Build Go Server
npm run build:go:mac  # or build:go:win for Windows

# 3. Start development mode
npm run dev
```

### Production (Real POS)

```bash
# Connect to real POS terminal via serial port
cd server_tcb && go run main.go -port /dev/ttyUSB0
```

## Protocol Specification

This project implements the TCB POS RS232 Protocol (Ver 3.6).

- `docs/TCB/TCB_RS232.md`
- `docs/TCB/TCB_FSM.md`
- `docs/TCB/tcb_layout_v36.json`

### Frame Structure (603 bytes)

| Position | Length | Field | Value |
|----------|--------|-------|-------|
| 0 | 1 | STX | `0x02` |
| 1-600 | 600 | DATA | ASCII payload |
| 601 | 1 | ETX | `0x03` |
| 602 | 1 | LRC | XOR checksum |

### Supported Transactions

| Command | Code | Description |
|---------|------|-------------|
| **SALE** | `01` | Credit card sale |
| **REFUND** | `02` | Refund transaction |
| **SETTLEMENT** | `50` | Daily batch settlement |
| **GETPAN** | `60` | Read card (2-step) |
| **COMPLETE** | `62` | Complete transaction (2-step) |
| **TERMINATE** | `70` | Cancel 2-step |
| **INQUIRY** | `07` | Timeout inquiry |

## API Reference

### WebSocket Endpoint

`ws://localhost:8989/ws`

### Request Format

```json
{
  "command": "SALE",
  "amount": "100",
  "host_id": "02",
  "trans_type": "01",
  "fields": {
    "Invoice_No": "000001"
  }
}
```

### Response Format

```json
{
  "status": "success",
  "message": "Transaction Approved",
  "data": {
    "TransType": "01",
    "Amount": "000000000100",
    "ApprovalNo": "123456",
    "OrderNo": "000001",
    "RespCode": "0000"
  }
}
```

### Status Values

| Status | Description |
|--------|-------------|
| `processing` | Transaction in progress |
| `success` | Transaction approved |
| `error` | Transaction failed |

### Response Codes (ECR)

| Code | Meaning |
|------|---------|
| `0000` | Approved |
| `0001` | Declined |
| `0011` | Timeout (use inquiry) |
| `0013` | Timeout (use inquiry) |

## Project Structure

```
ECPay-Server/
├── docs/
│   └── TCB/                 # TCB protocol specification & architecture
├── server_tcb/
│   ├── main.go               # Entry point
│   ├── api/                  # WebSocket handlers
│   ├── config/               # Configuration
│   ├── driver/               # Port abstraction (Serial/TCP)
│   ├── logger/               # Logging
│   └── protocol/             # TCB packet building/parsing
├── mock-pos-tcb/
│   └── main.go               # Mock POS simulator (TCB)
├── webapp/
│   ├── src/
│   │   ├── App.tsx           # Main application
│   │   ├── components/       # UI components
│   │   └── hooks/            # React hooks
│   └── package.json
├── electron-app/             # Electron desktop application
│   ├── src/
│   │   ├── main/             # Main process (Node.js)
│   │   │   ├── index.ts      # Entry point
│   │   │   ├── process-manager.ts
│   │   │   ├── websocket-bridge.ts
│   │   │   └── preload.ts
│   │   └── renderer/         # Renderer process (React)
│   ├── resources/bin/        # Go Server binary
│   └── package.json
└── README.md
```

## Development

### Building

```bash
# Build server
cd server_tcb && go build -o tcb-server

# Build mock-pos
cd mock-pos-tcb && go build -o mock-pos-tcb

# Build webapp
cd webapp && npm run build
```

### Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-ws` | `:8989` | WebSocket server address |
| `-port` | `` | Serial port name (COM3, /dev/ttyUSB0, tcp://host:port) |
| `-baud` | `115200` | Serial baud rate |
| `-autodetect` | `false` | Auto-detect POS device (not recommended for TCB) |

### Serial Port Settings

| Parameter | Value |
|-----------|-------|
| Baud Rate | 115200 |
| Data Bits | 8 |
| Parity | None |
| Stop Bits | 1 |

## License

MIT
