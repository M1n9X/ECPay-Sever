# ECPay POS Server

A WebSocket-based gateway server that bridges web applications with ECPay POS terminals via RS232 serial communication.

## Architecture

```
┌─────────────┐    WebSocket    ┌─────────────┐    TCP/RS232    ┌─────────────┐
│   Webapp    │ ◄───:5173───►   │   Server    │ ◄───:9999───►   │  Mock POS   │
│  (React)    │                 │    (Go)     │                 │   (Go)      │
└─────────────┘                 └─────────────┘                 └─────────────┘
                                       │
                                       ▼ (Production)
                                ┌─────────────┐
                                │  Real POS   │
                                │ (RS232 EDC) │
                                └─────────────┘
```

## Components

| Component | Directory | Port | Description |
|-----------|-----------|------|-------------|
| **Server** | `server/` | `:8989` | Go WebSocket server bridging webapp to POS |
| **Mock POS** | `mock-pos/` | `:9999` | POS terminal simulator for development |
| **Webapp** | `webapp/` | `:5173` | React TypeScript frontend for POS operations |

## Quick Start

### Development (Mock Mode)

```bash
# 1. Start Mock POS (in terminal 1)
cd mock-pos && go run main.go

# 2. Start Server in mock mode (in terminal 2)
cd server && go run main.go -mock

# 3. Start Webapp (in terminal 3)
cd webapp && npm install && npm run dev
```

Open <http://localhost:5173> in your browser.

### Production (Real POS)

```bash
# Connect to real POS terminal via serial port
cd server && go run main.go -port /dev/ttyUSB0
```

## Protocol Specification

This project implements the [ECPay POS RS232 Protocol](docs/RS232.md).

### Frame Structure (603 bytes)

| Position | Length | Field | Value |
|----------|--------|-------|-------|
| 0 | 1 | STX | `0x02` |
| 1-600 | 600 | DATA | ASCII payload |
| 601 | 1 | ETX | `0x03` |
| 602 | 1 | LRC | XOR checksum |

### Supported Transactions

| Type | Code | Description |
|------|------|-------------|
| **SALE** | `01` | Credit card sale |
| **REFUND** | `02` | Refund transaction |
| **SETTLEMENT** | `50` | Daily batch settlement |
| **ECHO** | `80` | Connection test |

### Key Data Fields

| Offset | Length | Field |
|--------|--------|-------|
| 0-1 | 2 | Trans Type |
| 2-3 | 2 | Host ID |
| 31-42 | 12 | Amount (no decimal) |
| 55-60 | 6 | Approval Number |
| 61-64 | 4 | Response Code |
| 88-107 | 20 | Order Number |
| 492-505 | 14 | POS Request Time |
| 506-545 | 40 | SHA-1 Hash |

## API Reference

### WebSocket Endpoint

`ws://localhost:8989/ws`

### Request Format

```json
{
  "command": "SALE",
  "amount": "100",
  "order_no": ""
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
    "OrderNo": "MOCK20260116095137",
    "CardNo": "4311****1234",
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

### Response Codes

| Code | Meaning |
|------|---------|
| `0000` | Approved |
| `0001` | Declined |
| `0002` | Call Bank |
| `0003` | Communication Error |

## Project Structure

```
ECPay-Server/
├── docs/
│   └── RS232.md          # Protocol specification
├── server/
│   ├── main.go           # Entry point
│   ├── api/              # WebSocket handlers
│   ├── config/           # Configuration
│   ├── driver/           # Port abstraction (Serial/TCP)
│   └── protocol/         # ECPay packet building/parsing
├── mock-pos/
│   └── main.go           # Mock POS simulator
├── webapp/
│   ├── src/
│   │   ├── App.tsx       # Main application
│   │   ├── components/   # UI components
│   │   └── hooks/        # React hooks (usePOS, useOrders)
│   └── package.json
└── README.md
```

## Development

### Building

```bash
# Build server
cd server && go build -o ecpay-server

# Build mock-pos
cd mock-pos && go build -o mock-pos

# Build webapp
cd webapp && npm run build
```

### Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `COM3` | Serial port name |
| `-mock` | `false` | Enable mock mode (TCP instead of serial) |

### Serial Port Settings

| Parameter | Value |
|-----------|-------|
| Baud Rate | 115200 |
| Data Bits | 8 |
| Parity | None |
| Stop Bits | 1 |

## License

MIT
