# ECPay POS System Architecture

## Overview

The ECPay POS system consists of three main components that work together to process credit card transactions via RS232 protocol:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              System Overview                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐    WebSocket     ┌──────────┐    RS232/TCP   ┌──────────┐ │
│  │          │ ◄──────────────► │          │ ◄────────────► │          │ │
│  │  Webapp  │    JSON msgs     │  Server  │   603-byte     │   POS    │ │
│  │  (React) │                  │   (Go)   │    frames      │ Terminal │ │
│  │          │                  │          │                │          │ │
│  └──────────┘                  └──────────┘                └──────────┘ │
│    :5173                         :8989                        :9999     │
│                                                            (mock mode)  │
└─────────────────────────────────────────────────────────────────────────┘
```

## Component Details

### 1. Webapp (React + TypeScript)

**Port:** 5173 (Vite dev server)

The webapp provides a modern POS terminal interface with:

- Amount input via numeric keypad
- SALE and REFUND transaction modes
- Real-time transaction status display
- Order history with refund capability
- Server status monitoring and control

### 2. Server (Go)

**Port:** 8989 (WebSocket)

The server bridges the webapp and POS terminal:

- WebSocket API for webapp communication
- RS232 protocol implementation for POS
- State machine for transaction management
- Logging with auto-rotation

### 3. POS Terminal / Mock POS

**Port:** 9999 (Mock mode only)

Physical ECPay POS terminal or mock simulator for testing:

- RS232 communication at 115200 bps
- 603-byte frame protocol (STX + 600B DATA + ETX + LRC)
- Transaction processing with card reader

---

## WebSocket Protocol

### Connection

```
ws://localhost:8989/ws
```

### Message Format

**Request (Webapp → Server):**

```json
{
  "command": "SALE" | "REFUND" | "STATUS" | "ABORT" | "RECONNECT" | "RESTART",
  "amount": "100",        // Amount in cents (for SALE/REFUND)
  "order_no": "ORD123"    // Original order number (for REFUND)
}
```

**Response (Server → Webapp):**

```json
{
  "status": "processing" | "success" | "error" | "status_update",
  "message": "Human readable message",
  "command_type": "transaction" | "control" | "status",
  "data": {
    // For transactions:
    "TransType": "01",           // 01=SALE, 02=REFUND
    "Amount": "100",
    "ApprovalNo": "123456",
    "OrderNo": "ORD20240116...",
    "CardNo": "************1234",
    "RespCode": "00",
    
    // For status updates:
    "state": "WAIT_RESPONSE",
    "elapsed_ms": 5000,
    "timeout_ms": 65000,
    "is_connected": true
  }
}
```

### Command Types

| Command | Type | Description |
|---------|------|-------------|
| `SALE` | transaction | Process credit card sale |
| `REFUND` | transaction | Process refund for previous order |
| `SETTLEMENT` | transaction | End-of-day settlement |
| `ECHO` | transaction | Connection test |
| `STATUS` | status | Request current server state |
| `ABORT` | control | Cancel in-progress transaction |
| `RECONNECT` | control | Reconnect to POS terminal |
| `RESTART` | control | Restart server (emergency) |

---

## State Machines

### Server State Machine

```
┌────────────────────────────────────────────────────────────────────────┐
│                        Server Transaction States                        │
├────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│    ┌──────┐  StartTransaction  ┌─────────┐                             │
│    │ IDLE │ ─────────────────► │ SENDING │                             │
│    └──────┘                    └────┬────┘                             │
│        ▲                            │                                   │
│        │                            ▼                                   │
│        │                      ┌──────────┐                             │
│        │                      │ WAIT_ACK │──────┐                      │
│        │                      └────┬─────┘      │                      │
│        │                           │            │                      │
│        │                           ▼            │                      │
│        │                    ┌──────────────┐    │                      │
│        │                    │ WAIT_RESPONSE│────┤                      │
│        │                    └──────┬───────┘    │                      │
│        │                           │            │                      │
│        │                           ▼            │                      │
│        │                      ┌─────────┐       │                      │
│        │                      │ PARSING │───────┤                      │
│        │                      └────┬────┘       │                      │
│        │                           │            │                      │
│        │              ┌────────────┼────────────┤                      │
│        │              ▼            ▼            ▼                      │
│        │         ┌─────────┐  ┌─────────┐  ┌─────────┐                │
│        └─────────│ SUCCESS │  │  ERROR  │  │ TIMEOUT │                │
│          Reset   └─────────┘  └─────────┘  └─────────┘                │
│                                                                         │
└────────────────────────────────────────────────────────────────────────┘
```

**Server States (from `driver/state.go`):**

| State | Timeout | Description |
|-------|---------|-------------|
| `IDLE` | - | Ready for new transaction |
| `SENDING` | 2s | Sending request packet to POS |
| `WAIT_ACK` | 5s | Waiting for ACK (0x06) from POS |
| `WAIT_RESPONSE` | 65s | Waiting for response (card operation) |
| `PARSING` | 2s | Parsing response packet |
| `SUCCESS` | - | Transaction approved |
| `ERROR` | - | Transaction failed |
| `TIMEOUT` | - | Operation timed out |

### Webapp State Machine

```
┌────────────────────────────────────────────────────────────────────────┐
│                         Webapp Application States                       │
├────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│    ┌──────────────┐   Connect    ┌──────┐                              │
│    │ DISCONNECTED │ ───────────► │ IDLE │ ◄───────────────────────┐    │
│    └──────────────┘              └──┬───┘                         │    │
│           ▲                         │                             │    │
│           │                         │ Submit Transaction          │    │
│           │ Disconnect              ▼                             │    │
│           │                   ┌────────────┐                      │    │
│           │                   │ PROCESSING │                      │    │
│           │                   └─────┬──────┘                      │    │
│           │                         │                             │    │
│           │         ┌───────────────┼───────────────┐             │    │
│           │         ▼               ▼               ▼             │    │
│           │    ┌─────────┐    ┌─────────┐    ┌─────────┐         │    │
│           │    │ SUCCESS │    │  ERROR  │    │ TIMEOUT │         │    │
│           │    └────┬────┘    └────┬────┘    └────┬────┘         │    │
│           │         │              │              │               │    │
│           │         └──────────────┴──────────────┘               │    │
│           │                        │ Dismiss                      │    │
│           │                        └──────────────────────────────┘    │
│           │                                                            │
└───────────┴────────────────────────────────────────────────────────────┘
```

**State Mapping (Server → Webapp):**

| Server State(s) | Webapp State | UI Behavior |
|-----------------|--------------|-------------|
| - (WebSocket closed) | `DISCONNECTED` | Show reconnect prompt |
| `IDLE` | `IDLE` | Enable form input |
| `SENDING`, `WAIT_ACK`, `WAIT_RESPONSE`, `PARSING` | `PROCESSING` | Show progress modal |
| `SUCCESS` | `SUCCESS` | Show success modal |
| `ERROR` | `ERROR` | Show error modal |
| `TIMEOUT` | `TIMEOUT` | Show timeout modal |

---

## Transaction Flow

### Successful SALE Transaction

```
    Webapp                    Server                      POS
      │                         │                          │
      │  1. Send SALE request   │                          │
      │ ───────────────────────►│                          │
      │                         │                          │
      │  2. status_update       │  3. Send 603-byte frame  │
      │     state: SENDING      │ ────────────────────────►│
      │ ◄───────────────────────│                          │
      │                         │                          │
      │  4. status_update       │  5. Receive ACK (0x06)   │
      │     state: WAIT_ACK     │ ◄────────────────────────│
      │ ◄───────────────────────│                          │
      │                         │                          │
      │  6. status_update       │                          │
      │     state: WAIT_RESPONSE│    (User inserts card)   │
      │ ◄───────────────────────│                          │
      │                         │                          │
      │                         │  7. Receive response     │
      │                         │ ◄────────────────────────│
      │                         │                          │
      │  8. status_update       │  9. Send ACK (0x06)      │
      │     state: PARSING      │ ────────────────────────►│
      │ ◄───────────────────────│                          │
      │                         │                          │
      │  10. success response   │                          │
      │      command_type:      │                          │
      │        transaction      │                          │
      │ ◄───────────────────────│                          │
      │                         │                          │
```

### Abort Transaction

```
    Webapp                    Server                      POS
      │                         │                          │
      │  (Transaction in progress - WAIT_RESPONSE)         │
      │                         │                          │
      │  1. Send ABORT          │                          │
      │ ───────────────────────►│                          │
      │                         │  2. Close cancel channel │
      │                         │     (internal signal)    │
      │                         │                          │
      │  3. success response    │                          │
      │     command_type:       │                          │
      │       control           │                          │
      │     "Transaction        │                          │
      │      aborted"           │                          │
      │ ◄───────────────────────│                          │
      │                         │                          │
      │  4. status_update       │                          │
      │     state: ERROR        │                          │
      │     "aborted by user"   │                          │
      │ ◄───────────────────────│                          │
      │                         │                          │
```

---

## Button State Logic

The webapp determines button states from the current `AppState`:

| Button | Enabled Condition |
|--------|-------------------|
| **Charge / Refund** | `state === IDLE && connected && amount > 0 && (tab !== REFUND \|\| orderNo)` |
| **Abort** | `state === PROCESSING` |
| **Keypad buttons** | `state === IDLE && connected` |
| **Tab buttons (SALE/REFUND)** | `state === IDLE && connected` |
| **POS Reconnect** | Always (control command) |
| **Restart Server** | Always (emergency command) |

---

## File Structure

```
ECPay-Server/
├── server/                     # Go server
│   ├── main.go                 # Entry point
│   ├── api/
│   │   └── websocket.go        # WebSocket handler
│   ├── driver/
│   │   ├── state.go            # State machine
│   │   ├── manager.go          # Transaction manager
│   │   ├── serial.go           # Serial port interface
│   │   └── tcp.go              # TCP mock interface
│   ├── protocol/
│   │   └── packet.go           # RS232 packet builder
│   ├── logger/
│   │   └── logger.go           # Logging with rotation
│   └── config/
│       └── config.go           # Configuration
│
├── webapp/                     # React webapp
│   ├── src/
│   │   ├── App.tsx             # Main component
│   │   ├── hooks/
│   │   │   ├── useAppState.ts  # State machine hook
│   │   │   ├── usePOS.ts       # WebSocket hook
│   │   │   └── useOrders.ts    # Order history
│   │   └── components/
│   │       ├── Keypad.tsx      # Numeric keypad
│   │       ├── OrderHistory.tsx # Transaction list
│   │       └── ServerStatus.tsx # Status panel
│   └── package.json
│
├── mock-pos/                   # Mock POS simulator
│   └── main.go
│
├── docs/                       # Documentation
│   ├── RS232.md                # Protocol specification
│   └── architecture.md         # This document
│
├── start.sh                    # Start all services
└── stop.sh                     # Stop all services
```

---

## Error Handling

### Server-side Errors

| Error Type | Behavior | Recovery |
|------------|----------|----------|
| Serial port error | `ERROR` state | Use reconnect |
| ACK timeout (5s) | `TIMEOUT` state | Retry transaction |
| Response timeout (65s) | `TIMEOUT` state | Retry transaction |
| LRC checksum mismatch | `ERROR` state | Retry transaction |
| Transaction in progress | Reject new request | Wait or abort |

### Webapp-side Errors

| Error Type | Behavior | Recovery |
|------------|----------|----------|
| WebSocket disconnected | `DISCONNECTED` state | Auto-reconnect |
| Parse error | Log to console | Ignore message |
| Invalid response | Show error modal | Dismiss and retry |

---

## Logging

### Server Logs

Location: `server/log/ecpay-server.log`

**Log Levels:**

- `[INFO]` - General operations
- `[ERROR]` - Errors and failures
- `[DEBUG]` - Detailed debugging
- `[TRANS]` - Transaction events
- `[PROTO]` - Protocol-level data

**Auto-rotation:** When `log/` directory exceeds 10MB, old logs are deleted.

### Webapp Logs

- Displayed in ServerStatus panel (last 50 entries)
- Also output to browser console

---

## Quick Start

```bash
# Start all services
./start.sh

# Access webapp
open http://localhost:5173

# Stop all services
./stop.sh
```

**Manual Start:**

```bash
# Terminal 1: Mock POS
cd mock-pos && go run main.go

# Terminal 2: Server
cd server && go run main.go -mock

# Terminal 3: Webapp
cd webapp && npm run dev
```
