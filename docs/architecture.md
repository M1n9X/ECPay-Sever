# ECPay POS System Architecture

## Overview

The ECPay POS system consists of three main components that work together to process credit card transactions via RS232 protocol.

### Development Environment

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Development Environment                          │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐      TCP :9999      ┌──────────┐   WebSocket  ┌─────────┐ │
│  │ Mock POS │ ◄─────────────────► │  Server  │ ◄──────────► │ Webapp  │ │
│  │  (Go)    │   tcp://localhost   │   (Go)   │   :8989/ws   │ (React) │ │
│  │          │                     │          │              │         │ │
│  └──────────┘                     └──────────┘              └─────────┘ │
│                                                                          │
│  * Mock POS listens on TCP :9999                                        │
│  * Server auto-detects via ECHO handshake (same as production)          │
│  * Scanner probes tcp://localhost:9999 alongside COM ports              │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### Production Environment (Real Serial)

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Production Environment                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐   RS232 Serial      ┌──────────┐   WebSocket  ┌─────────┐ │
│  │ ECPay    │ ◄─────────────────► │  Server  │ ◄──────────► │ Webapp  │ │
│  │ POS      │   COM3 / ttyUSB0    │   (Go)   │   :8989/ws   │ (React) │ │
│  │ Terminal │   115200 bps 8N1    │          │              │         │ │
│  └──────────┘                     └──────────┘              └─────────┘ │
│                                                                          │
│  * Server auto-detects COM port via ECHO handshake                      │
│  * Same auto-detection logic as development                             │
│  * TCP endpoint (localhost:9999) simply won't respond                   │
│                                                                          │
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

**Key Files:**
- `src/hooks/useAppState.ts` - State machine
- `src/hooks/usePOS.ts` - WebSocket communication
- `src/hooks/useOrders.ts` - Order history management

### 2. Server (Go)

**Port:** 8989 (WebSocket)

The server bridges the webapp and POS terminal:

- WebSocket API for webapp communication
- RS232 protocol implementation (603-byte frames)
- Auto-detection of POS devices via ECHO handshake
- State machine for transaction management
- Logging with auto-rotation

**Key Files:**
- `driver/scanner.go` - Auto-detection with ECHO handshake
- `driver/manager.go` - Transaction execution
- `driver/serial.go` - Serial port abstraction
- `driver/state.go` - State machine
- `protocol/packet.go` - RS232 frame builder
- `api/websocket.go` - WebSocket handler

### 3. Mock POS (Go)

**Modes:** PTY (development) or TCP (fallback)

Mock simulator for development and testing:

- Creates virtual serial port via socat PTY
- Full RS232 protocol implementation
- Configurable delays, error rates, decline rates
- Responds to ECHO handshake for auto-detection

**Key Features:**
- `-mode pty` - Virtual serial port (recommended)
- `-mode tcp` - TCP fallback (port 9999)
- `-delay 2000` - Processing delay in ms
- `-decline-prob 0.1` - 10% decline rate
- `-verbose` - Detailed logging

---

## Auto-Detection Flow

The Server automatically discovers POS devices using ECHO handshake:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Auto-Detection Sequence                           │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Scanner                                                                 │
│     │                                                                    │
│     ├─► discoverPorts()                                                  │
│     │      ├─► serial.GetPortsList()      // Hardware: COM3, ttyUSB0    │
│     │      └─► Add tcp://localhost:9999   // Mock POS endpoint          │
│     │                                                                    │
│     └─► For each port: probePort()                                       │
│            │                                                             │
│            ├─► OpenSerial(port, 115200)   // Works for both serial/TCP  │
│            ├─► Send ECHO request (TransType=80)                          │
│            ├─► Wait for ACK (500ms timeout)                              │
│            ├─► Wait for Response (3s timeout)                            │
│            ├─► Validate LRC checksum                                     │
│            ├─► Verify TransType=80 in response                           │
│            ├─► Send ACK to complete handshake                            │
│            └─► ConnectTo(port) if successful                             │
│                                                                          │
│  Timing:                                                                 │
│    - Initial burst: 3 attempts, 1s apart                                │
│    - Periodic scan: Every 20s if disconnected                           │
│                                                                          │
│  Port Types:                                                             │
│    - Serial: COM3, /dev/ttyUSB0, /dev/cu.usbserial-*                    │
│    - TCP: tcp://localhost:9999 (Mock POS)                               │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

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
  "amount": "100",
  "order_no": "ORD123"
}
```

**Response (Server → Webapp):**

```json
{
  "status": "processing" | "success" | "error" | "status_update",
  "message": "Human readable message",
  "command_type": "transaction" | "control" | "status",
  "data": {
    "TransType": "01",
    "Amount": "100",
    "ApprovalNo": "123456",
    "OrderNo": "ORD20240116...",
    "CardNo": "************1234",
    "RespCode": "0000",
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
| `RECONNECT` | control | Trigger POS device rescan |
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
│        │                           │            │ Error/Timeout        │
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

**State Timeouts:**

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

**State Mapping (Server → Webapp):**

| Server State(s) | Webapp State | UI Behavior |
|-----------------|--------------|-------------|
| (WebSocket closed) | `DISCONNECTED` | Show reconnect prompt |
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
      │  1. SALE request        │                          │
      │ ───────────────────────►│                          │
      │                         │                          │
      │  2. status: SENDING     │  3. Send 603-byte frame  │
      │ ◄───────────────────────│ ────────────────────────►│
      │                         │                          │
      │  4. status: WAIT_ACK    │  5. ACK (0x06)           │
      │ ◄───────────────────────│ ◄────────────────────────│
      │                         │                          │
      │  6. status: WAIT_RESPONSE                          │
      │ ◄───────────────────────│    (User swipes card)    │
      │                         │                          │
      │                         │  7. Response frame       │
      │                         │ ◄────────────────────────│
      │                         │                          │
      │  8. status: PARSING     │  9. ACK (0x06)           │
      │ ◄───────────────────────│ ────────────────────────►│
      │                         │                          │
      │  10. success response   │                          │
      │ ◄───────────────────────│                          │
```

---

## File Structure

```
ECPay-Server/
├── server/                     # Go server (production-ready)
│   ├── main.go                 # Entry point
│   ├── api/
│   │   └── websocket.go        # WebSocket handler
│   ├── driver/
│   │   ├── state.go            # State machine
│   │   ├── manager.go          # Transaction manager
│   │   ├── scanner.go          # Auto-detection with ECHO handshake
│   │   └── serial.go           # Serial port interface
│   ├── protocol/
│   │   ├── packet.go           # RS232 frame builder
│   │   ├── parser.go           # Response parser
│   │   └── crypto.go           # LRC & SHA-1
│   ├── logger/
│   │   └── logger.go           # Logging with rotation
│   └── config/
│       └── config.go           # Configuration
│
├── webapp/                     # React webapp
│   ├── src/
│   │   ├── App.tsx             # Main component
│   │   ├── hooks/
│   │   │   ├── useAppState.ts  # State machine
│   │   │   ├── usePOS.ts       # WebSocket communication
│   │   │   └── useOrders.ts    # Order history
│   │   └── components/
│   │       ├── Keypad.tsx      # Numeric keypad
│   │       ├── OrderHistory.tsx
│   │       └── ServerStatus.tsx
│   └── package.json
│
├── mock-pos/                   # Mock POS simulator
│   ├── main.go                 # TCP mode, full RS232 protocol
│   └── go.mod
│
├── docs/
│   ├── RS232.md                # ECPay RS232 protocol spec
│   └── architecture.md         # This document
│
├── start.sh                    # Start all services (PTY mode)
└── stop.sh                     # Stop all services
```

---

## Dependencies

### Runtime

| Component | Dependencies |
|-----------|--------------|
| Server | Go 1.21+, go.bug.st/serial |
| Webapp | Node.js 18+, npm |
| Mock POS | Go 1.21+ |

---

## Quick Start

### Development (with Mock POS)

```bash
# Start all services
./start.sh

# Access webapp
open http://localhost:5173

# Stop all services
./stop.sh
```

### Production (Windows)

```bash
# Build server
cd server && go build -o ecpay-server.exe .

# Run (auto-detects COM port via ECHO handshake)
./ecpay-server.exe
```

### Manual Start (Development)

```bash
# Terminal 1: Mock POS
cd mock-pos && ./mock-pos

# Terminal 2: Server (auto-detects Mock POS on tcp://localhost:9999)
cd server && ./ecpay-server

# Terminal 3: Webapp
cd webapp && npm run dev
```

---

## Error Handling

### Server-side Errors

| Error Type | State | Recovery |
|------------|-------|----------|
| Port open failed | - | Auto-retry via scanner |
| Write error | ERROR | Trigger rescan |
| ACK timeout (5s) | TIMEOUT | Retry transaction |
| Response timeout (65s) | TIMEOUT | Retry transaction |
| LRC checksum mismatch | ERROR | Retry transaction |
| NAK received | ERROR | Check POS status |
| Transaction in progress | - | Wait or abort |

### Connection Recovery

```
Connection Lost
      │
      ▼
  SetConnected(false)
      │
      ▼
  Trigger Scanner
      │
      ▼
  scanAndConnect()
      │
      ├─► Found: ConnectTo(port)
      │
      └─► Not Found: Retry in 20s
```

---

## Logging

### Server Logs

**Location:** `server/log/ecpay-server.YYYY-MM-DD.log`

**Log Levels:**
- `[INFO]` - General operations
- `[WARN]` - Warnings
- `[ERROR]` - Errors and failures
- `[DEBUG]` - Detailed debugging
- `[TRANS]` - Transaction events

**Auto-rotation:** Logs older than 30 days are deleted when directory exceeds 10MB.

### Webapp Logs

- Displayed in ServerStatus panel (last 50 entries)
- Also output to browser console
